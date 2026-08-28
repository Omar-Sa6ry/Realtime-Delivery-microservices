import { Inject, Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { KafkaEventEnvelope } from '@delivery/common';
import { KafkaEventHandler, DELIVERY_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';

@Injectable()
export class KafkaConsumer implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(KafkaConsumer.name);
  constructor(@Inject(DELIVERY_EVENT_HANDLERS) private readonly handlers: KafkaEventHandler[]) {}
  async onModuleInit(): Promise<void> { this.logger.log(`Delivery Kafka consumer initialized with ${this.handlers.length} handler(s)`); }
  async onModuleDestroy(): Promise<void> { this.logger.log('Delivery Kafka consumer stopped'); }
  async process(envelope: KafkaEventEnvelope): Promise<void> {
    const handler = this.handlers.find((candidate) => candidate.eventType === envelope.eventType);
    if (handler) await handler.handle(envelope);
  }
}
