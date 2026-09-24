import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { KafkaModule } from '@delivery/common';
import { KafkaConsumer } from './kafka.consumer';
import { REALTIME_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';
import { DeliveryCreatedHandler } from './handlers/delivery-created.handler';
import { DriverAssignedHandler } from './handlers/driver-assigned.handler';
import { DriverAcceptedHandler } from './handlers/driver-accepted.handler';
import { DeliveryPickedUpHandler } from './handlers/delivery-picked-up.handler';
import { DeliveryInTransitHandler } from './handlers/delivery-in-transit.handler';
import { DeliveryCompletedHandler } from './handlers/delivery-completed.handler';
import { DeliveryCancelledHandler } from './handlers/delivery-cancelled.handler';
import { PaymentCompletedHandler } from './handlers/payment-completed.handler';
import { PaymentFailedHandler } from './handlers/payment-failed.handler';
import { DriverAssignmentOfferedHandler } from './handlers/driver-assignment-offered.handler';
import { DriverAssignmentRejectedHandler } from './handlers/driver-assignment-rejected.handler';
import { DriverAssignmentExpiredHandler } from './handlers/driver-assignment-expired.handler';
import { DriverNoDriverAvailableHandler } from './handlers/driver-no-driver-available.handler';

@Module({
  imports: [
    KafkaModule.registerAsync({
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        clientId: config.get<string>('KAFKA_CLIENT_ID', 'realtime-service'),
        brokers: config
          .get<string>('KAFKA_BROKERS', 'localhost:9092')
          .split(',')
          .map((b) => b.trim())
          .filter(Boolean),
      }),
      inject: [ConfigService],
    }),
  ],
  providers: [
    KafkaConsumer,
    DeliveryCreatedHandler,
    DriverAssignedHandler,
    DriverAcceptedHandler,
    DeliveryPickedUpHandler,
    DeliveryInTransitHandler,
    DeliveryCompletedHandler,
    DeliveryCancelledHandler,
    PaymentCompletedHandler,
    PaymentFailedHandler,
    DriverAssignmentOfferedHandler,
    DriverAssignmentRejectedHandler,
    DriverAssignmentExpiredHandler,
    DriverNoDriverAvailableHandler,
    {
      provide: REALTIME_EVENT_HANDLERS,
      useFactory: (
        deliveryCreated: DeliveryCreatedHandler,
        driverAssigned: DriverAssignedHandler,
        driverAccepted: DriverAcceptedHandler,
        deliveryPickedUp: DeliveryPickedUpHandler,
        deliveryInTransit: DeliveryInTransitHandler,
        deliveryCompleted: DeliveryCompletedHandler,
        deliveryCancelled: DeliveryCancelledHandler,
        paymentCompleted: PaymentCompletedHandler,
        paymentFailed: PaymentFailedHandler,
        driverAssignmentOffered: DriverAssignmentOfferedHandler,
        driverAssignmentRejected: DriverAssignmentRejectedHandler,
        driverAssignmentExpired: DriverAssignmentExpiredHandler,
        driverNoDriverAvailable: DriverNoDriverAvailableHandler,
      ) => [
        deliveryCreated,
        driverAssigned,
        driverAccepted,
        deliveryPickedUp,
        deliveryInTransit,
        deliveryCompleted,
        deliveryCancelled,
        paymentCompleted,
        paymentFailed,
        driverAssignmentOffered,
        driverAssignmentRejected,
        driverAssignmentExpired,
        driverNoDriverAvailable,
      ],
      inject: [
        DeliveryCreatedHandler,
        DriverAssignedHandler,
        DriverAcceptedHandler,
        DeliveryPickedUpHandler,
        DeliveryInTransitHandler,
        DeliveryCompletedHandler,
        DeliveryCancelledHandler,
        PaymentCompletedHandler,
        PaymentFailedHandler,
        DriverAssignmentOfferedHandler,
        DriverAssignmentRejectedHandler,
        DriverAssignmentExpiredHandler,
        DriverNoDriverAvailableHandler,
      ],
    },
  ],
  exports: [KafkaConsumer],
})
export class KafkaConsumerModule {}