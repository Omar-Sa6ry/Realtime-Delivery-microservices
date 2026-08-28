import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Delivery } from './entities/delivery.entity';
import { DeliveryStatusHistory } from './entities/delivery-status-history.entity';
import { Outbox } from './entities/outbox.entity';
import { DeliverySagaState } from './entities/delivery-saga-state.entity';

@Module({
  imports: [TypeOrmModule.forFeature([Delivery, DeliveryStatusHistory, Outbox, DeliverySagaState])],
  exports: [TypeOrmModule],
})
export class DeliveryModule {}
