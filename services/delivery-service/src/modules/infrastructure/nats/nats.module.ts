import { Module } from '@nestjs/common';
import { NatsClientModule } from '@delivery-micro/shard';

@Module({
  imports: [
    NatsClientModule.register({
      name: 'DELIVERY_NATS',
      queue: process.env.NATS_QUEUE ?? 'delivery-service',
    }),
  ],
  exports: [NatsClientModule],
})
export class DeliveryNatsModule {}
