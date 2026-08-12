/**
 * Media service event types for Kafka / NATS consumers.
 * Any NestJS service that subscribes to media events should import from here.
 */

export enum MediaEventType {
  UploadCreated       = 'media.upload.created',
  UploadCompleted     = 'media.upload.completed',
  UploadAborted       = 'media.upload.aborted',

  ScanStarted         = 'media.scan.started',
  ScanCompleted       = 'media.scan.completed',
  ScanFailed          = 'media.scan.failed',

  ProcessingStarted   = 'media.processing.started',
  ProcessingCompleted = 'media.processing.completed',
  ProcessingFailed    = 'media.processing.failed',

  MediaReady          = 'media.ready',

  DeleteRequested     = 'media.delete.requested',
  MediaDeleted        = 'media.deleted',
  DeleteFailed        = 'media.delete.failed',
}

export interface MediaVersionInfo {
  versionType: string;
  objectKey: string;
  contentType: string;
  size: number;
  width?: number;
  height?: number;
  durationMs?: number;
}

export interface MediaUploadCreatedPayload {
  mediaId: string;
  userId: string;
  deliveryId?: string;
  fileName: string;
  contentType: string;
  mediaType: string;
  size: number;
  uploadId: string;
}

export interface MediaUploadCompletedPayload {
  mediaId: string;
  userId: string;
  objectKey: string;
  size: number;
  contentType: string;
  mediaType: string;
}

export interface MediaReadyPayload {
  mediaId: string;
  userId: string;
  mediaType: string;
  contentType: string;
  versions: MediaVersionInfo[];
}

export interface MediaDeletedPayload {
  mediaId: string;
  userId: string;
  at: string; // ISO 8601
}

export interface MediaScanCompletedPayload {
  mediaId: string;
  userId: string;
  infected: boolean;
  threat?: string;
}

/**
 * Generic Kafka event envelope — matches the Go EventEnvelope struct.
 * All media events are wrapped in this structure.
 */
export interface MediaEventEnvelope<T = unknown> {
  eventId: string;
  eventType: MediaEventType;
  traceId?: string;
  timestamp: number; // unix milliseconds
  payload: T;
}
