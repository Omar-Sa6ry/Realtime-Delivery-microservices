import { Test, TestingModule } from '@nestjs/testing';
import { OutboxPublisherService } from './outbox-publisher.service';
import { OutboxRepository } from './outbox.repository';
import { KafkaProducer } from './kafka.producer';
import { Outbox, OutboxStatus } from '../entities/outbox.entity';
import { mockOutbox, createMockOutboxRepository, createMockKafkaProducer } from '../../../../test-utils';

describe('OutboxPublisherService', () => {
  let service: OutboxPublisherService;
  let outboxRepository: ReturnType<typeof createMockOutboxRepository>;
  let kafkaProducer: ReturnType<typeof createMockKafkaProducer>;

  beforeEach(async () => {
    outboxRepository = createMockOutboxRepository();
    kafkaProducer = createMockKafkaProducer();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        OutboxPublisherService,
        { provide: OutboxRepository, useValue: outboxRepository },
        { provide: KafkaProducer, useValue: kafkaProducer },
      ],
    }).compile();

    service = module.get<OutboxPublisherService>(OutboxPublisherService);
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
  });

  describe('onModuleInit', () => {
    it('should set up interval and call publishBatch', async () => {
      const publishBatchSpy = jest.spyOn(service as any, 'publishBatch').mockResolvedValue(undefined);
      outboxRepository.claimPending.mockResolvedValue([]);

      await service.onModuleInit();

      expect(publishBatchSpy).toHaveBeenCalled();
      expect(setInterval).toHaveBeenCalled();
    });
  });

  describe('onModuleDestroy', () => {
    it('should clear interval', async () => {
      const clearIntervalSpy = jest.spyOn(global, 'clearInterval');

      await service.onModuleInit();
      await service.onModuleDestroy();

      expect(clearIntervalSpy).toHaveBeenCalled();
    });
  });

  describe('publishBatch', () => {
    it('should not run if already running', async () => {
      (service as any).running = true;
      outboxRepository.claimPending.mockResolvedValue([]);

      await service.publishBatch();

      expect(outboxRepository.claimPending).not.toHaveBeenCalled();
    });

    it('should process claimed events', async () => {
      const events = [mockOutbox({ id: '1' }), mockOutbox({ id: '2' })];
      outboxRepository.claimPending.mockResolvedValue(events);
      const publishSpy = jest.spyOn(service as any, 'publish').mockResolvedValue(undefined);

      await service.publishBatch();

      expect(outboxRepository.claimPending).toHaveBeenCalled();
      expect(publishSpy).toHaveBeenCalledTimes(2);
      expect((service as any).running).toBe(false);
    });

    it('should set running to false even if publish throws', async () => {
      const events = [mockOutbox({ id: '1' })];
      outboxRepository.claimPending.mockResolvedValue(events);
      jest.spyOn(service as any, 'publish').mockRejectedValue(new Error('Publish failed'));

      await expect(service.publishBatch()).rejects.toThrow('Publish failed');
      expect((service as any).running).toBe(false);
    });
  });

  describe('publish', () => {
    it('should publish event and mark as published', async () => {
      const event = mockOutbox({ status: OutboxStatus.PROCESSING });
      kafkaProducer.publish.mockResolvedValue(undefined);
      outboxRepository.markPublished.mockResolvedValue({ ...event, status: OutboxStatus.PUBLISHED });

      await (service as any).publish(event);

      expect(kafkaProducer.publish).toHaveBeenCalledWith(event);
      expect(outboxRepository.markPublished).toHaveBeenCalledWith(event);
    });

    it('should mark as failed when publish fails', async () => {
      const event = mockOutbox({ status: OutboxStatus.PROCESSING, attempts: 1 });
      const error = new Error('Kafka error');
      kafkaProducer.publish.mockRejectedValue(error);
      outboxRepository.markFailed.mockResolvedValue({ ...event, status: OutboxStatus.FAILED });

      await (service as any).publish(event);

      expect(kafkaProducer.publish).toHaveBeenCalledWith(event);
      expect(outboxRepository.markFailed).toHaveBeenCalledWith(event, error, expect.any(Number));
    });

    it('should calculate exponential backoff for retry delay', async () => {
      const event = mockOutbox({ status: OutboxStatus.PROCESSING, attempts: 3 });
      const error = new Error('Kafka error');
      kafkaProducer.publish.mockRejectedValue(error);
      outboxRepository.markFailed.mockResolvedValue(event);

      await (service as any).publish(event);

      const expectedDelay = Math.min(300000, 1000 * 2 ** Math.min(3, 8));
      expect(outboxRepository.markFailed).toHaveBeenCalledWith(event, error, expectedDelay);
    });

    it('should cap retry delay at 5 minutes', async () => {
      const event = mockOutbox({ status: OutboxStatus.PROCESSING, attempts: 10 });
      const error = new Error('Kafka error');
      kafkaProducer.publish.mockRejectedValue(error);
      outboxRepository.markFailed.mockResolvedValue(event);

      await (service as any).publish(event);

      expect(outboxRepository.markFailed).toHaveBeenCalledWith(event, error, 300000);
    });
  });

  describe('intervalMs', () => {
    it('should use default interval', () => {
      const interval = (service as any).intervalMs;
      expect(interval).toBeGreaterThanOrEqual(250);
    });

    it('should use environment variable when set', () => {
      process.env.OUTBOX_PUBLISH_INTERVAL_MS = '5000';
      const service2 = new OutboxPublisherService(outboxRepository, kafkaProducer);
      expect((service2 as any).intervalMs).toBe(5000);
      delete process.env.OUTBOX_PUBLISH_INTERVAL_MS;
    });

    it('should enforce minimum interval', () => {
      process.env.OUTBOX_PUBLISH_INTERVAL_MS = '100';
      const service2 = new OutboxPublisherService(outboxRepository, kafkaProducer);
      expect((service2 as any).intervalMs).toBe(250);
      delete process.env.OUTBOX_PUBLISH_INTERVAL_MS;
    });
  });

  describe('batchSize', () => {
    it('should use default batch size', () => {
      expect((service as any).batchSize).toBe(25);
    });

    it('should use environment variable when set', () => {
      process.env.OUTBOX_BATCH_SIZE = '50';
      const service2 = new OutboxPublisherService(outboxRepository, kafkaProducer);
      expect((service2 as any).batchSize).toBe(50);
      delete process.env.OUTBOX_BATCH_SIZE;
    });

    it('should enforce batch size limits', () => {
      process.env.OUTBOX_BATCH_SIZE = '200';
      const service2 = new OutboxPublisherService(outboxRepository, kafkaProducer);
      expect((service2 as any).batchSize).toBe(100);
      delete process.env.OUTBOX_BATCH_SIZE;

      process.env.OUTBOX_BATCH_SIZE = '0';
      const service3 = new OutboxPublisherService(outboxRepository, kafkaProducer);
      expect((service3 as any).batchSize).toBe(1);
      delete process.env.OUTBOX_BATCH_SIZE;
    });
  });
});