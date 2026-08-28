import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Delivery } from './entities/delivery.entity';
import { DeliveryStatusHistory } from './entities/delivery-status-history.entity';
import { Outbox } from './entities/outbox.entity';
import { DeliverySagaState } from './entities/delivery-saga-state.entity';
import { DeliveryRepository } from './repositories/delivery.repository';
import { DeliveryStateMachine } from './services/delivery.state-machine';
import { DeliveryCommandService } from './services/delivery-command.service';
import { DeliveryQueryService } from './services/delivery-query.service';
import { IdempotencyService } from './services/idempotency.service';
import { DeliveryResolver } from './graphql/delivery.resolver';
import { DeliveryQueryResolver } from './graphql/delivery.query.resolver';
import { AppResolver } from './graphql/app.resolver';
import { OutboxRepository } from './outbox/outbox.repository';
import { OutboxPublisherService } from './outbox/outbox-publisher.service';
import { KafkaProducer } from './outbox/kafka.producer';
import { DeliverySagaOrchestrator } from './saga/delivery-saga.orchestrator';
import { PaymentConfirmationStep } from './saga/steps/payment-confirmation.step';
import { DriverAssignmentStep } from './saga/steps/driver-assignment.step';

@Module({
  imports: [
    TypeOrmModule.forFeature([
      Delivery,
      DeliveryStatusHistory,
      Outbox,
      DeliverySagaState,
    ]),
  ],
  providers: [
    DeliveryRepository,
    DeliveryStateMachine,
    DeliveryCommandService,
    DeliveryQueryService,
    IdempotencyService,
    DeliveryResolver,
    DeliveryQueryResolver,
    AppResolver,
    OutboxRepository,
    OutboxPublisherService,
    KafkaProducer,
  ],
  exports: [
    TypeOrmModule,
    DeliveryRepository,
    DeliveryStateMachine,
    DeliveryCommandService,
    DeliveryQueryService,
    IdempotencyService,
    OutboxRepository,
    OutboxPublisherService,
    KafkaProducer,
  ],
})
export class DeliveryModule {}


