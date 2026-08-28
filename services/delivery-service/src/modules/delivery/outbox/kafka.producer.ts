import { Injectable } from '@nestjs/common';
import { KafkaService } from '@delivery/common';
import { Outbox } from '../entities/outbox.entity';

@Injectable()
export class KafkaProducer {
  constructor(private readonly kafka: KafkaService) {}
  publish(event: Outbox): Promise<void> {
    return this.kafka.emit(event.eventType, event.eventType, event.payload, {
      key: event.aggregateId,
    });
  }
}
