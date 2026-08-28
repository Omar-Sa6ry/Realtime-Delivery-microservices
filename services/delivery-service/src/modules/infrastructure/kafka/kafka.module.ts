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

@Module({
  imports: [
    KafkaModule.registerAsync({
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        clientId: config.get<string>('KAFKA_CLIENT_ID', 'delivery-service'),
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
    {
      provide: DELIVERY_EVENT_HANDLERS,
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
      ],
    },
  ],
  exports: [KafkaConsumer],
})
export class KafkaConsumerModule {}