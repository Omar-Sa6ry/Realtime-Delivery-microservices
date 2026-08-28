export interface KafkaEventEnvelope<T = unknown> {
  eventId: string;
  eventType: string;
  aggregateId: string;
  aggregateType: string;
  version: number;
  timestamp: number;
  traceId?: string;
  payload: T;
}
