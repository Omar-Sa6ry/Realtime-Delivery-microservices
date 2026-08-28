import { Module } from '@nestjs/common';
import { KafkaConsumer } from './kafka.consumer';
import { DELIVERY_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';

@Module({
  providers: [KafkaConsumer, { provide: DELIVERY_EVENT_HANDLERS, useValue: [] }],
  exports: [KafkaConsumer],
})
export class KafkaConsumerModule {}
