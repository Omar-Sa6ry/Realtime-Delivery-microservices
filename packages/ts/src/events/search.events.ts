/**
 * Search service events used for transient NATS signals and durable analytics events.
 */
export enum SearchEventType {
  QueryStarted = 'search.query.started',
  QueryCompleted = 'search.query.completed',
  ReindexStarted = 'search.reindex.started',
  ReindexCompleted = 'search.reindex.completed',
  ReindexFailed = 'search.reindex.failed',
}

export interface SearchQueryStartedPayload {
  queryHash: string;
  index: string;
  userId?: string;
  traceId?: string;
  startedAt: string;
}

export interface SearchQueryCompletedPayload {
  queryHash: string;
  index: string;
  userId?: string;
  traceId?: string;
  latencyMs: number;
  resultCount: number;
  cacheHit: boolean;
  zeroResults: boolean;
  completedAt: string;
}

export interface SearchReindexStartedPayload {
  jobId: string;
  index: string;
  startedAt: string;
  triggeredBy: string;
}

export interface SearchReindexCompletedPayload {
  jobId: string;
  index: string;
  documentsTotal: number;
  documentsFailed: number;
  durationMs: number;
  completedAt: string;
}

export interface SearchReindexFailedPayload {
  jobId: string;
  index: string;
  error: string;
  failedAt: string;
}

export interface SearchEventEnvelope<T = unknown> {
  eventId: string;
  eventType: SearchEventType | string;
  traceId?: string;
  timestamp: number;
  payload: T;
}
