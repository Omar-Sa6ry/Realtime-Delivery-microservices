import { Test } from '@nestjs/testing';
import { getRepositoryToken } from '@nestjs/typeorm';
import { ConfigService } from '@nestjs/config';
import { ClientProxyFactory } from '@nestjs/microservices';
import { OutboxProcessor } from './outbox.queue';
import { Outbox } from '../database/entities/outbox.entity';

jest.mock('@nestjs/microservices', () => ({
  ...jest.requireActual('@nestjs/microservices'),
  ClientProxyFactory: { create: jest.fn() },
}));

describe('OutboxProcessor', () => {
  const client = {
    connect: jest.fn().mockResolvedValue(undefined),
    emit: jest.fn().mockReturnValue(undefined),
  };
  const repo = {
    findOne: jest.fn(),
    save: jest.fn().mockResolvedValue(undefined),
  };
  const config = {
    get: jest.fn((key: string, fallback?: string) =>
      key === 'NATS_URL' ? 'nats://nats-srv:4222' : fallback,
    ),
  };

  let processor: OutboxProcessor;

  beforeEach(async () => {
    jest.clearAllMocks();
    client.connect.mockResolvedValue(undefined);
    repo.save.mockResolvedValue(undefined);
    (ClientProxyFactory.create as jest.Mock).mockReturnValue(client);

    const moduleRef = await Test.createTestingModule({
      providers: [
        OutboxProcessor,
        { provide: getRepositoryToken(Outbox), useValue: repo },
        { provide: ConfigService, useValue: config },
      ],
    }).compile();

    processor = moduleRef.get(OutboxProcessor);
  });

  /** Builds a BullMQ shaped job carrying the outbox id. */
  function job(outboxId: string) {
    return { data: { outboxId } } as any;
  }

  function storedEvent(overrides: Partial<Outbox> = {}): Outbox {
    return {
      id: 'outbox-1',
      eventType: 'user.created',
      payload: { id: 'user-1' },
      aggregateId: 'user-1',
      processed: false,
      ...overrides,
    } as unknown as Outbox;
  }

  it('creates a NATS client bound to the configured server', () => {
    expect(ClientProxyFactory.create).toHaveBeenCalledWith({
      transport: expect.anything(),
      options: { servers: ['nats://nats-srv:4222'], queue: 'user-service' },
    });
    expect(config.get).toHaveBeenCalledWith(
      'NATS_URL',
      'nats://localhost:4222',
    );
  });

  it('falls back to the default NATS url when nothing is configured', async () => {
    config.get.mockImplementation((key: string, fallback?: string) =>
      key === 'NATS_URL' ? fallback : undefined,
    );

    const moduleRef = await Test.createTestingModule({
      providers: [
        OutboxProcessor,
        { provide: getRepositoryToken(Outbox), useValue: repo },
        { provide: ConfigService, useValue: config },
      ],
    }).compile();

    moduleRef.get(OutboxProcessor);

    expect(ClientProxyFactory.create).toHaveBeenCalledWith(
      expect.objectContaining({
        options: {
          servers: ['nats://localhost:4222'],
          queue: 'user-service',
        },
      }),
    );
  });

  it('publishes the event and marks the outbox row as processed', async () => {
    const event = storedEvent();
    repo.findOne.mockResolvedValue(event);

    await processor.process(job('outbox-1'));

    expect(repo.findOne).toHaveBeenCalledWith({ where: { id: 'outbox-1' } });
    expect(client.connect).toHaveBeenCalledTimes(1);
    expect(client.emit).toHaveBeenCalledWith('user.created', {
      id: 'user-1',
    });
    expect(event.processed).toBe(true);
    expect(repo.save).toHaveBeenCalledWith(event);
  });

  it('does nothing when the outbox row is missing', async () => {
    repo.findOne.mockResolvedValue(null);

    await expect(processor.process(job('outbox-404'))).resolves.toBeUndefined();

    expect(client.connect).not.toHaveBeenCalled();
    expect(client.emit).not.toHaveBeenCalled();
    expect(repo.save).not.toHaveBeenCalled();
  });

  it('does nothing when the event was already processed', async () => {
    repo.findOne.mockResolvedValue(storedEvent({ processed: true }));

    await expect(processor.process(job('outbox-1'))).resolves.toBeUndefined();

    expect(client.connect).not.toHaveBeenCalled();
    expect(client.emit).not.toHaveBeenCalled();
    expect(repo.save).not.toHaveBeenCalled();
  });

  it('rethrows a publish failure so BullMQ can retry the job', async () => {
    const failure = new Error('nats down');
    repo.findOne.mockResolvedValue(storedEvent());
    client.connect.mockRejectedValue(failure);

    await expect(processor.process(job('outbox-1'))).rejects.toThrow(
      'nats down',
    );

    expect(repo.save).not.toHaveBeenCalled();
  });

  it('rethrows a persistence failure after the event was emitted', async () => {
    const failure = new Error('disk full');
    repo.findOne.mockResolvedValue(storedEvent());
    repo.save.mockRejectedValue(failure);

    await expect(processor.process(job('outbox-1'))).rejects.toThrow(
      'disk full',
    );

    expect(client.emit).toHaveBeenCalledTimes(1);
  });
});
