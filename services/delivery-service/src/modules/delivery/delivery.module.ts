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
  ],
  exports: [
    TypeOrmModule,
    DeliveryRepository,
    DeliveryStateMachine,
    DeliveryCommandService,
    DeliveryQueryService,
    IdempotencyService,
  ],
})
export class DeliveryModule {}
