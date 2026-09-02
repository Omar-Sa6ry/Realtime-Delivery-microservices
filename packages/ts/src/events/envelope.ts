/**
 * Generic cross-domain event envelope matching the Go EventEnvelope contract.
 */
export interface EventEnvelope<T = unknown> {
  eventId: string;
  eventType: string;
  traceId?: string;
  timestamp: number;
  payload: T;
}
