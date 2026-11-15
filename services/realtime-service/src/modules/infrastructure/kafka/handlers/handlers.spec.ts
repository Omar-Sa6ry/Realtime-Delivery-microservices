import {
  DeliveryKafkaTopics,
  DriverEventType,
  PaymentKafkaTopics,
  RealtimeNatsSubjects,
} from '@delivery-micro/shard';
import { EventDeduplicator } from '../../../features/events/event-deduplicator';
import { EventMapper } from '../../../features/events/event.mapper';
import { NatsPublisher } from '../../nats/nats.publisher';
import { RealtimeMetricsService } from '../../../../common/metrics/realtime-metrics.service';
import { DeliveryCreatedHandler } from './delivery-created.handler';
import { DriverAssignedHandler } from './driver-assigned.handler';
import { DriverAcceptedHandler } from './driver-accepted.handler';
import { DeliveryPickedUpHandler } from './delivery-picked-up.handler';
import { DeliveryInTransitHandler } from './delivery-in-transit.handler';
import { DeliveryCompletedHandler } from './delivery-completed.handler';
import { DeliveryCancelledHandler } from './delivery-cancelled.handler';
import { PaymentCompletedHandler } from './payment-completed.handler';
import { PaymentFailedHandler } from './payment-failed.handler';
import { DriverAssignmentOfferedHandler } from './driver-assignment-offered.handler';
import { DriverAssignmentRejectedHandler } from './driver-assignment-rejected.handler';
import { DriverAssignmentExpiredHandler } from './driver-assignment-expired.handler';
import { DriverNoDriverAvailableHandler } from './driver-no-driver-available.handler';

describe('Concrete Kafka event handlers', () => {
  const deduplicator = { isDuplicate: jest.fn().mockResolvedValue(false) };
  const mapper = {
    toClientEvent: jest.fn().mockReturnValue({ type: 'E', data: {} }),
  };
  const natsPublisher = { publish: jest.fn().mockResolvedValue(true) };
  const metrics = { kafkaProcessed: { inc: jest.fn() } };

  const deps = () => [
    deduplicator as unknown as EventDeduplicator,
    mapper as unknown as EventMapper,
    natsPublisher as unknown as NatsPublisher,
    metrics as unknown as RealtimeMetricsService,
  ];

  beforeEach(() => {
    jest.clearAllMocks();
    deduplicator.isDuplicate.mockResolvedValue(false);
    natsPublisher.publish.mockResolvedValue(true);
  });

  it.each([
    [
      DeliveryCreatedHandler,
      DeliveryKafkaTopics.DELIVERY_CREATED,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    ],
    [
      DriverAssignedHandler,
      DeliveryKafkaTopics.DRIVER_ASSIGNED,
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED,
    ],
    [
      DriverAcceptedHandler,
      DeliveryKafkaTopics.DRIVER_ACCEPTED,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    ],
    [
      DeliveryPickedUpHandler,
      DeliveryKafkaTopics.DELIVERY_PICKED_UP,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    ],
    [
      DeliveryInTransitHandler,
      DeliveryKafkaTopics.DELIVERY_IN_TRANSIT,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    ],
    [
      DeliveryCompletedHandler,
      DeliveryKafkaTopics.DELIVERY_COMPLETED,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    ],
    [
      DeliveryCancelledHandler,
      DeliveryKafkaTopics.DELIVERY_CANCELLED,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    ],
    [
      PaymentCompletedHandler,
      PaymentKafkaTopics.PAYMENT_COMPLETED,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    ],
    [
      PaymentFailedHandler,
      PaymentKafkaTopics.PAYMENT_FAILED,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    ],
    [
      DriverAssignmentOfferedHandler,
      DriverEventType.AssignmentOffered,
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED,
    ],
    [
      DriverAssignmentRejectedHandler,
      DriverEventType.AssignmentRejected,
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED,
    ],
    [
      DriverAssignmentExpiredHandler,
      DriverEventType.AssignmentExpired,
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED,
    ],
    [
      DriverNoDriverAvailableHandler,
      DriverEventType.NoDriverAvailable,
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED,
    ],
  ])(
    '%s binds eventType and natsSubject and handles events',
    (Ctor: any, eventType, natsSubject) => {
      const handler = new Ctor(...deps());

      expect(handler.eventType).toBe(eventType);
      expect(handler.natsSubject).toBe(natsSubject);
    },
  );

  it('end-to-end handles a delivery.created event through the template', async () => {
    const handler = new DeliveryCreatedHandler(...deps());

    await handler.handle({
      eventId: 'evt-1',
      eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
      timestamp: Date.now(),
      payload: { deliveryId: 'd1', status: 'CREATED' },
    } as any);

    expect(mapper.toClientEvent).toHaveBeenCalled();
    expect(natsPublisher.publish).toHaveBeenCalledWith(
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
      { type: 'E', data: {} },
    );
    expect(metrics.kafkaProcessed.inc).toHaveBeenCalledWith({
      eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
    });
  });

  it('all handlers are unique per eventType', () => {
    const handlers = [
      new DeliveryCreatedHandler(...deps()),
      new DriverAssignedHandler(...deps()),
      new DriverAcceptedHandler(...deps()),
      new DeliveryPickedUpHandler(...deps()),
      new DeliveryInTransitHandler(...deps()),
      new DeliveryCompletedHandler(...deps()),
      new DeliveryCancelledHandler(...deps()),
      new PaymentCompletedHandler(...deps()),
      new PaymentFailedHandler(...deps()),
      new DriverAssignmentOfferedHandler(...deps()),
      new DriverAssignmentRejectedHandler(...deps()),
      new DriverAssignmentExpiredHandler(...deps()),
      new DriverNoDriverAvailableHandler(...deps()),
    ];

    const types = handlers.map((h) => h.eventType);
    expect(new Set(types).size).toBe(13);
  });
});
