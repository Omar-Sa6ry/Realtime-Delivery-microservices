export interface EventEnvelope<T = unknown> {
  eventId: string;
  eventType: string;
  traceId?: string;
  timestamp: number;
  payload: T;
}
