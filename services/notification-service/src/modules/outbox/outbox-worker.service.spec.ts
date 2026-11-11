import { OutboxWorkerService } from './outbox-worker.service';
import {
  NotificationOutbox,
  OutboxStatus,
} from '../../common/database/entities/notification-outbox.entity';
import { createMockRepository, MockRepository } from '../../test-utils';
import { ConfigService } from '@nestjs/config';
import { AlertService } from '@delivery-micro/shard';
import { ClientProxy } from '@nestjs/microservices';

const mockNatsClient = {
  connect: jest.fn().mockResolvedValue(undefined),
  close: jest.fn().mockResolvedValue(undefined),
  emit: jest.fn().mockReturnValue(undefined),
  on: jest.fn(),
};

jest.mock('@nestjs/microservices', () => {
  const actual = jest.requireActual('@nestjs/microservices');
  return {
    ...actual,
    ClientProxyFactory: {
      create: jest.fn(() => mockNatsClient),
    },
  };
});

describe('OutboxWorkerService', () => {
  let service: OutboxWorkerService;
  let outboxRepository: MockRepository<NotificationOutbox>;
  let configService: { get: jest.Mock };
  let alerts: { triggerAlert: jest.Mock };

  const pendingRecord = (
    overrides: Partial<NotificationOutbox> = {},
  ): NotificationOutbox =>
    ({
      id: 'out-1',
      eventType: 'NOTIFICATION_CREATED',
      aggregateId: 'n-1',
      payload: { userId: 'user-1', notificationId: 'n-1' },
      status: OutboxStatus.PENDING,
      attemptCount: 0,
      createdAt: new Date(),
      ...overrides,
    }) as NotificationOutbox;

  beforeEach(() => {
    jest.clearAllMocks();
    outboxRepository = createMockRepository<NotificationOutbox>();
    configService = {
      get: jest.fn().mockReturnValue('nats://localhost:4222'),
    };
    alerts = { triggerAlert: jest.fn().mockResolvedValue(true) };

    service = new OutboxWorkerService(
      outboxRepository as any,
      configService as unknown as ConfigService,
      alerts as unknown as AlertService,
    );
  });

  afterEach(async () => {
    await service.onModuleDestroy();
  });

  describe('lifecycle', () => {
    it('connects to NATS and starts the processing timer on init', async () => {
      await service.onModuleInit();

      expect(mockNatsClient.connect).toHaveBeenCalled();
      expect((service as any).timer).toBeDefined();
    });

    it('processes pending records when the interval fires', async () => {
      jest.useFakeTimers();
      const record = pendingRecord();
      outboxRepository.find.mockResolvedValue([record]);
      outboxRepository.save.mockImplementation(async (r: any) => r);

      await service.onModuleInit();
      jest.advanceTimersByTime(5000);
      await Promise.resolve();

      expect(mockNatsClient.emit).toHaveBeenCalledWith(
        'notification.user.user-1',
        record.payload,
      );
      expect(record.status).toBe(OutboxStatus.PUBLISHED);

      await service.onModuleDestroy();
      jest.useRealTimers();
    });

    it('clears the timer and closes the NATS client on destroy', async () => {
      await service.onModuleInit();
      const clearIntervalSpy = jest.spyOn(global, 'clearInterval');

      await service.onModuleDestroy();

      expect(clearIntervalSpy).toHaveBeenCalled();
      expect(mockNatsClient.close).toHaveBeenCalled();
    });
  });

  describe('processOutbox', () => {
    it('publishes pending records on the per-user NATS subject and marks them PUBLISHED', async () => {
      const record = pendingRecord();
      outboxRepository.find.mockResolvedValue([record]);
      outboxRepository.save.mockImplementation(async (r: any) => r);

      await (service as any).processOutbox();

      expect(outboxRepository.find).toHaveBeenCalledWith(
        expect.objectContaining({
          where: { status: OutboxStatus.PENDING },
          take: 50,
          order: { createdAt: 'ASC' },
        }),
      );
      expect(mockNatsClient.emit).toHaveBeenCalledWith(
        'notification.user.user-1',
        record.payload,
      );
      expect(record.status).toBe(OutboxStatus.PUBLISHED);
      expect(record.publishedAt).toBeInstanceOf(Date);
      expect(outboxRepository.save).toHaveBeenCalledWith(record);
      expect(alerts.triggerAlert).not.toHaveBeenCalled();
    });

    it('does nothing when there are no pending records', async () => {
      outboxRepository.find.mockResolvedValue([]);

      await (service as any).processOutbox();

      expect(mockNatsClient.emit).not.toHaveBeenCalled();
    });

    it('increments attemptCount on publish failure', async () => {
      const record = pendingRecord({ attemptCount: 1 });
      outboxRepository.find.mockResolvedValue([record]);
      outboxRepository.save.mockImplementation(async (r: any) => r);
      mockNatsClient.emit.mockImplementation(() => {
        throw new Error('nats down');
      });

      await (service as any).processOutbox();

      expect(record.attemptCount).toBe(2);
      expect(record.status).toBe(OutboxStatus.PENDING);
      expect(outboxRepository.save).toHaveBeenCalledWith(record);
    });

    it('marks the record FAILED and fires a CRITICAL alert after more than 5 attempts', async () => {
      const record = pendingRecord({ attemptCount: 5 });
      outboxRepository.find.mockResolvedValue([record]);
      outboxRepository.save.mockImplementation(async (r: any) => r);
      mockNatsClient.emit.mockImplementation(() => {
        throw new Error('nats down');
      });

      await (service as any).processOutbox();

      expect(record.attemptCount).toBe(6);
      expect(record.status).toBe(OutboxStatus.FAILED);
      expect(alerts.triggerAlert).toHaveBeenCalledWith(
        expect.any(String),
        expect.stringContaining('failed after'),
        'CRITICAL',
      );
    });

    it('alerts WARNING when the whole outbox sweep throws', async () => {
      outboxRepository.find.mockRejectedValue(new Error('db down'));

      await (service as any).processOutbox();

      expect(alerts.triggerAlert).toHaveBeenCalledWith(
        expect.any(String),
        expect.stringContaining('Error processing outbox'),
        'WARNING',
      );
    });
  });

  it('creates the NATS client with the configured server', () => {
    expect(configService.get).toHaveBeenCalledWith(
      'NATS_URL',
      'nats://nats-srv:4222',
    );
    expect((service as any).natsClient).toBeDefined();
  });

  it('exposes the client proxy instance', () => {
    expect((service as any).natsClient).toBeInstanceOf(Object);
    expect((service as any).natsClient as ClientProxy).toBeDefined();
  });
});
