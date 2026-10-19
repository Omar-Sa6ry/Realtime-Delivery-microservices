import { Test, TestingModule } from '@nestjs/testing';
import { KafkaProducer } from './kafka.producer';
import { KafkaService } from '@delivery-micro/shard';
import { Outbox } from '../entities/outbox.entity';
import { createMockKafkaService, mockOutbox } from '../../../test-utils';

describe('KafkaProducer', () => {
  let producer: KafkaProducer;
  let kafkaService: ReturnType<typeof createMockKafkaService>;

  beforeEach(async () => {
    kafkaService = createMockKafkaService();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        KafkaProducer,
        { provide: KafkaService, useValue: kafkaService },
      ],
    }).compile();

    producer = module.get<KafkaProducer>(KafkaProducer);
  });

  it('should be defined', () => {
    expect(producer).toBeDefined();
  });

  describe('publish', () => {
    it('should publish event to Kafka', async () => {
      const event = mockOutbox({
        eventId: 'event-123',
        eventType: 'delivery.created',
        aggregateId: 'delivery-123',
        payload: { deliveryId: 'delivery-123', status: 'CREATED' },
      });

      await producer.publish(event);

      expect(kafkaService.emit).toHaveBeenCalledWith(
        'delivery.created',
        'delivery.created',
        { deliveryId: 'delivery-123', status: 'CREATED' },
        { key: 'delivery-123' }
      );
    });

    it('should use aggregateId as key', async () => {
      const event = mockOutbox({
        eventType: 'delivery.updated',
        aggregateId: 'delivery-456',
        payload: { deliveryId: 'delivery-456', status: 'DRIVER_ASSIGNED' },
      });

      await producer.publish(event);

      expect(kafkaService.emit).toHaveBeenCalledWith(
        'delivery.updated',
        'delivery.updated',
        { deliveryId: 'delivery-456', status: 'DRIVER_ASSIGNED' },
        { key: 'delivery-456' }
      );
    });

    it('should handle complex payload', async () => {
      const complexPayload = {
        deliveryId: 'delivery-123',
        customerId: 'customer-123',
        driverId: 'driver-123',
        status: 'PICKED_UP',
        amount: '100.00',
        currency: 'USD',
        pickup: { line1: '123 St', city: 'SF' },
        dropoff: { line1: '456 Ave', city: 'LA' },
        changedBy: 'driver-123',
        note: 'Picked up',
        updatedAt: new Date().toISOString(),
      };
      const event = mockOutbox({
        eventType: 'delivery.picked_up',
        aggregateId: 'delivery-123',
        payload: complexPayload,
      });

      await producer.publish(event);

      expect(kafkaService.emit).toHaveBeenCalledWith(
        'delivery.picked_up',
        'delivery.picked_up',
        complexPayload,
        { key: 'delivery-123' }
      );
    });
  });
});