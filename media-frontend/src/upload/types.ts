export type MediaStatus =
  | 'PENDING'
  | 'UPLOADING'
  | 'UPLOADED'
  | 'SCANNING'
  | 'PROCESSING'
  | 'READY'
  | 'FAILED'
  | 'QUARANTINED'
  | 'DELETING'
  | 'DELETED'
  | 'ABORTED';

export interface PresignedPart {
  partNumber: number;
  presignedUrl: string;
}

export interface UploadSession {
  mediaId: string;
  uploadId: string;
  s3UploadId: string;
  presignedParts: PresignedPart[];
  partSize: number;
  totalParts: number;
  expiresAt: number;
}

export interface CompletedPart {
  partNumber: number;
  eTag: string;
}

export interface User {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  role: string;
  imageUrl?: string | null;
}

export interface AuthPayload {
  accessToken: string;
  refreshToken: string;
  user: User;
}

export interface UploadProgressState {
  mediaId: string | null;
  uploadId: string | null;
  phase:
    | 'IDLE'
    | 'CREATING_SESSION'
    | 'UPLOADING'
    | 'COMPLETING'
    | 'LINKING_AVATAR'
    | 'DONE'
    | 'ERROR';
  uploadedBytes: number;
  totalBytes: number;
  uploadedParts: number;
  totalParts: number;
  status: MediaStatus | null;
  error?: string;
}

export interface MediaWsEvent {
  event:
    | 'SCAN_STARTED'
    | 'SCAN_COMPLETED'
    | 'SCAN_FAILED'
    | 'PROCESSING_STARTED'
    | 'PROCESSING_PROGRESS'
    | 'PROCESSING_COMPLETED'
    | 'PROCESSING_FAILED'
    | 'READY'
    | 'DELETE_STARTED'
    | 'DELETE_COMPLETED'
    | 'DELETE_FAILED';
  mediaId?: string;
  uploadId?: string;
  stage?: string;
  progress?: number;
}

export const WS_EVENT_TO_STATUS: Partial<Record<MediaWsEvent['event'], MediaStatus>> = {
  SCAN_STARTED: 'SCANNING',
  SCAN_COMPLETED: 'PROCESSING',
  SCAN_FAILED: 'FAILED',
  PROCESSING_STARTED: 'PROCESSING',
  PROCESSING_COMPLETED: 'PROCESSING',
  PROCESSING_FAILED: 'FAILED',
  READY: 'READY',
  DELETE_COMPLETED: 'DELETED',
};