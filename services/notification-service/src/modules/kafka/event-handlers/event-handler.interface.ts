export interface KafkaEventPayload {
  eventId?: string;
  id?: string;
  eventType?: string;
  userId?: string;
  type?: string;
  data?: Record<string, unknown>;
  payload?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface IEventHandler {
  handle(payload: KafkaEventPayload): Promise<void>;
}