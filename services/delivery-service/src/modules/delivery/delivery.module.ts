import { forwardRef, Module } from '@nestjs/common';
import { ClientsModule, Transport } from '@nestjs/microservices';
import { TypeOrmModule } from '@nestjs/typeorm';
import { join } from 'path';
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

import { RedisModule, RedisService } from '@bts-soft/core';
import { DeliveryKafkaModule } from '../infrastructure/kafka/kafka.module';
import { DeliveryNatsModule } from '../infrastructure/nats/nats.module';

@Module({
  imports: [
    RedisModule,
    forwardRef(() => DeliveryKafkaModule),
    DeliveryNatsModule,
    TypeOrmModule.forFeature([
      Delivery,
      DeliveryStatusHistory,
      Outbox,
      DeliverySagaState,
    ]),
    ClientsModule.register([
      {
        name: 'USER_SERVICE',
        transport: Transport.GRPC,
        options: {
          package: 'user',
          protoPath: join(process.cwd(), '../../protos/user.proto'),
          url: process.env.USER_SERVICE_URL || 'user-srv:50051',
        },
      },
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
    PaymentConfirmationStep,
    DriverAssignmentStep,
    DeliverySagaOrchestrator,
    {
      provide: 'SHARED_REDIS_SERVICE',
      useExisting: RedisService,
    },
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
    PaymentConfirmationStep,
    DriverAssignmentStep,
    DeliverySagaOrchestrator,
  ],
})
export class DeliveryModule {}
