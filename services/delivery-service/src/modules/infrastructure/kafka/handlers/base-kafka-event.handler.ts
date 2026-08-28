import { Injectable } from '@nestjs/common';
import { KafkaEventEnvelope } from '@delivery/common';

export interface KafkaEventHandler { readonly eventType: string; handle(envelope: KafkaEventEnvelope): Promise<void>; }
export const DELIVERY_EVENT_HANDLERS = 'DELIVERY_EVENT_HANDLERS';

@Injectable()
export abstract class BaseKafkaEventHandler<T = unknown> implements KafkaEventHandler {
  abstract readonly eventType: string;
  protected validate(payload: unknown): T { return payload as T; }
  async handle(envelope: KafkaEventEnvelope): Promise<void> { this.validate(envelope.payload); }
}
