import { Test, TestingModule } from '@nestjs/testing';
import { OutboxWorkerService } from './outbox-worker.service';
import { Repository } from 'typeorm';
import { Outbox } from '../database/entities/outbox.entity';
import { ClientProxy, ClientProxyFactory, Transport } from '@nestjs/microservices';
import { Queue } from 'bullmq';
import { ConfigService } from '@nestjs/config';

jest.mock('@nestjs/microservices', () => {
  const original = jest.requireActual('@nestjs/microservices');
  return {
    ...original,
    ClientProxyFactory: {
      create: jest.fn(),
    },
    Transport: original.Transport,
  };
});

describe('OutboxWorkerService', () => {
  let service: OutboxWorkerService;
  let outboxRepo: jest.Mocked<Repository<Outbox>>;
  let configService: jest.Mocked<ConfigService>;
  let outboxQueue: jest.Mocked<Queue>;
  let mockClient: jest.Mocked<ClientProxy>;
  let mockTransactionManager: any;
  let mockClientProxyFactory: jest.Mocked<typeof ClientProxyFactory>;

  const mockOutboxEvent: Outbox = {
    id: 'outbox-123',
    aggregateType: 'User',
    aggregateId: 'user-123',
    eventType: 'user.created',
    payload: { userId: 'user-123', email: 'test@example.com' },
    processed: false,
    createdAt: new Date(),
  } as Outbox;

  beforeEach(async () => {
    outboxRepo = {
      find: jest.fn(),
      save: jest.fn(),
      manager: {
        transaction: jest.fn(),
      },
    } as unknown as jest.Mocked<Repository<Outbox>>;

    configService = {
      get: jest.fn().mockReturnValue('nats://localhost:4222'),
    } as unknown as jest.Mocked<ConfigService>;

    outboxQueue = {
      add: jest.fn().mockResolvedValue({}),
    } as unknown as jest.Mocked<Queue>;

    mockClient = {
      connect: jest.fn().mockResolvedValue(undefined),
      emit: jest.fn().mockReturnValue(undefined),
      close: jest.fn(),
    } as unknown as jest.Mocked<ClientProxy>;

    mockTransactionManager = {
      find: jest.fn(),
      save: jest.fn(),
    };

    outboxRepo.manager.transaction.mockImplementation(async (callback) => {
      return callback(mockTransactionManager);
    });

    mockClientProxyFactory = ClientProxyFactory as jest.Mocked<typeof ClientProxyFactory>;
    mockClientProxyFactory.create.mockReturnValue(mockClient);

    jest.useFakeTimers();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        OutboxWorkerService,
        { provide: 'OutboxRepository', useValue: outboxRepo },
        { provide: ConfigService, useValue: configService },
        { provide: 'BullQueue_outbox-queue', useValue: outboxQueue },
      ],
    }).compile();

    service = module.get<OutboxWorkerService>(OutboxWorkerService);
  });

  afterEach(() => {
    jest.useRealTimers();
    jest.clearAllMocks();
  });

  describe('onModuleInit', () => {
    it('should connect to NATS', async () => {
      await service.onModuleInit();

      expect(mockClientProxyFactory.create).toHaveBeenCalledWith({
        transport: Transport.NATS,
        options: {
          servers: ['nats://localhost:4222'],
          queue: 'user-service',
        },
      });
      expect(mockClient.connect).toHaveBeenCalled();
    });

    it('should log warning if NATS connection fails', async () => {
      mockClient.connect.mockRejectedValueOnce(new Error('Connection refused'));

      await service.onModuleInit();

      expect(mockClient.connect).toHaveBeenCalled();
    });
  });

  describe('onModuleDestroy', () => {
    it('should close client', async () => {
      await service.onModuleInit();
      await service.onModuleDestroy();

      expect(mockClient.close).toHaveBeenCalled();
    });
  });

  describe('enqueueEvent', () => {
    it('should add event to BullMQ queue', async () => {
      await service.enqueueEvent('outbox-123');

      expect(outboxQueue.add).toHaveBeenCalledWith(
        'process-event',
        { outboxId: 'outbox-123' },
        { removeOnComplete: true },
      );
    });

    it('should log error if queue add fails', async () => {
      outboxQueue.add.mockRejectedValueOnce(new Error('Redis error'));

      await service.enqueueEvent('outbox-123');

      expect(outboxQueue.add).toHaveBeenCalled();
    });
  });

  describe('reconcileOutbox', () => {
    beforeEach(() => {
      mockClient.emit.mockReturnValue(undefined);
    });

    it('should process unprocessed events', async () => {
      mockTransactionManager.find.mockResolvedValue([mockOutboxEvent]);
      mockTransactionManager.save.mockResolvedValue({ ...mockOutboxEvent, processed: true });

      await service.reconcileOutbox();

      expect(mockTransactionManager.find).toHaveBeenCalledWith(Outbox, {
        where: { processed: false },
        order: { createdAt: 'ASC' },
        take: 20,
        lock: { mode: 'pessimistic_write', onLocked: 'skip_locked' },
      });
      expect(mockClient.connect).toHaveBeenCalled();
      expect(mockClient.emit).toHaveBeenCalledWith('user.created', mockOutboxEvent.payload);
      expect(mockTransactionManager.save).toHaveBeenCalledWith(
        expect.objectContaining({ processed: true }),
      );
    });

    it('should handle empty outbox', async () => {
      mockTransactionManager.find.mockResolvedValue([]);

      await service.reconcileOutbox();

      expect(mockTransactionManager.find).toHaveBeenCalled();
      expect(mockClient.emit).not.toHaveBeenCalled();
    });

    it('should handle multiple events', async () => {
      const events = [
        { ...mockOutboxEvent, id: 'outbox-1', eventType: 'user.created' },
        { ...mockOutboxEvent, id: 'outbox-2', eventType: 'user.updated' },
      ];
      mockTransactionManager.find.mockResolvedValue(events);
      mockTransactionManager.save.mockResolvedValue({ processed: true });

      await service.reconcileOutbox();

      expect(mockClient.emit).toHaveBeenCalledTimes(2);
      expect(mockTransactionManager.save).toHaveBeenCalledTimes(2);
    });

    it('should handle database errors gracefully', async () => {
      outboxRepo.manager.transaction.mockRejectedValueOnce(new Error('DB error'));

      await expect(service.reconcileOutbox()).resolves.toBeUndefined();
    });
  });
});