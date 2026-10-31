import { useCallback, useEffect, useRef, useState } from 'react';
import {
  completeUpload,
  createUploadSession,
  updateProfileAvatar,
} from '../api/operations';
import { sha256Hex, uploadFileParts } from './presignedUpload';
import type { MediaStatus, UploadProgressState } from './types';
import { mediaSocket } from './websocket';

const IDLE: UploadProgressState = {
  mediaId: null,
  uploadId: null,
  phase: 'IDLE',
  uploadedBytes: 0,
  totalBytes: 0,
  uploadedParts: 0,
  totalParts: 0,
  status: null,
};

/**
 * Orchestrates: createUploadSession → direct S3 presigned PUT → completeUpload →
 * updateProfile(avatarMediaId) (resolved by user-service over gRPC).
 */
export function useUploadAvatar(accessToken: string | null): {
  progress: UploadProgressState;
  uploadAvatar: (file: File) => Promise<{ mediaId: string }>;
  reset: () => void;
} {
  const [progress, setProgress] = useState<UploadProgressState>(IDLE);
  const accessTokenRef = useRef(accessToken);
  accessTokenRef.current = accessToken;

  useEffect(() => {
    if (!accessToken) return;
    const off = mediaSocket.onEvent((event) => {
      if (!event.mediaId) return;
      setProgress((p) => {
        if (p.mediaId && p.mediaId !== event.mediaId) return p;
        return { ...p, status: mediaSocketStatus(event) ?? p.status };
      });
    });
    mediaSocket.connect(accessToken);
    return () => {
      off();
      mediaSocket.disconnect();
    };
  }, [accessToken]);

  const uploadAvatar = useCallback(async (file: File) => {
    const checksum = await sha256Hex(file);
    setProgress({
      ...IDLE,
      phase: 'CREATING_SESSION',
      totalBytes: file.size,
      status: 'PENDING',
    });

    const session = await createUploadSession({
      fileName: file.name,
      contentType: file.type || 'application/octet-stream',
      size: file.size,
      checksum,
    });

    setProgress({
      mediaId: session.mediaId,
      uploadId: session.uploadId,
      phase: 'UPLOADING',
      uploadedBytes: 0,
      totalBytes: file.size,
      uploadedParts: 0,
      totalParts: session.totalParts,
      status: 'UPLOADING',
    });

    const parts = await uploadFileParts(file, session, {
      onProgress: (uploadedBytes) =>
        setProgress((p) => ({ ...p, uploadedBytes })),
      onPartComplete: (partNumber) =>
        setProgress((p) => ({ ...p, uploadedParts: Math.max(p.uploadedParts, partNumber) })),
    });

    setProgress((p) => ({ ...p, phase: 'COMPLETING' }));
    const { mediaId } = await completeUpload(session.uploadId, parts);

    setProgress((p) => ({
      ...p,
      mediaId,
      phase: 'LINKING_AVATAR',
      status: 'UPLOADED',
    }));

    // user-service resolves the avatar URL via gRPC → media-service (ownership check)
    await updateProfileAvatar(mediaId);

    setProgress((p) => ({ ...p, phase: 'DONE', status: p.status ?? 'PROCESSING' }));
    return { mediaId };
  }, []);

  const reset = useCallback(() => setProgress(IDLE), []);

  return { progress, uploadAvatar, reset };
}

function mediaSocketStatus(event: { event: string }): MediaStatus | null {
  switch (event.event) {
    case 'SCAN_STARTED':
      return 'SCANNING';
    case 'SCAN_COMPLETED':
    case 'PROCESSING_STARTED':
      return 'PROCESSING';
    case 'SCAN_FAILED':
    case 'PROCESSING_FAILED':
      return 'FAILED';
    case 'READY':
      return 'READY';
    case 'DELETE_COMPLETED':
      return 'DELETED';
    default:
      return null;
  }
}