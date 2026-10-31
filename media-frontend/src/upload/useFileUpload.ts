import { useCallback, useRef, useState } from 'react';
import {
  completeUpload,
  createUploadSession,
} from '../api/operations';
import { sha256Hex, uploadFileParts } from './presignedUpload';
import type { CompletedPart, MediaStatus, UploadProgressState } from './types';

// ─── allowed types ────────────────────────────────────────────────────────────
export const ALLOWED_TYPES: Record<string, string> = {
  'image/jpeg':       'image',
  'image/png':        'image',
  'image/webp':       'image',
  'image/gif':        'image',
  'image/avif':       'image',
  'video/mp4':        'video',
  'video/quicktime':  'video',
  'video/x-msvideo':  'video',
  'video/x-matroska': 'video',
  'video/webm':       'video',
  'application/pdf':  'document',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document': 'document',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet':       'document',
  'text/plain':       'document',
  'application/zip':  'document',
};

const MAX_BYTES: Record<string, number> = {
  image:    10  * 1024 * 1024,
  video:    50  * 1024 * 1024 * 1024,
  document: 500 * 1024 * 1024,
};

export function getFileKind(type: string): string | null {
  return ALLOWED_TYPES[type] ?? null;
}

export function validateFile(file: File): string | null {
  const kind = getFileKind(file.type);
  if (!kind) return `نوع الملف "${file.type}" غير مدعوم`;
  if (file.size > MAX_BYTES[kind])
    return `حجم الملف يتخطى الحد المسموح (${formatBytes(MAX_BYTES[kind])})`;
  return null;
}

export function formatBytes(b: number): string {
  if (b >= 1e9) return `${(b / 1e9).toFixed(1)} GB`;
  if (b >= 1e6) return `${(b / 1e6).toFixed(1)} MB`;
  if (b >= 1e3) return `${(b / 1e3).toFixed(1)} KB`;
  return `${b} B`;
}

// ─── upload item state ────────────────────────────────────────────────────────

export interface UploadItem {
  /** Client-side UUID for React key */
  id: string;
  file: File;
  kind: string;
  previewUrl: string | null;
  validationError: string | null;
  progress: UploadProgressState;
  /** Resolved S3 URL once status is READY */
  resultUrl: string | null;
  aborted: boolean;
}

const IDLE_PROGRESS: UploadProgressState = {
  mediaId: null,
  uploadId: null,
  phase: 'IDLE',
  uploadedBytes: 0,
  totalBytes: 0,
  uploadedParts: 0,
  totalParts: 0,
  status: null,
};

function makeItem(file: File): UploadItem {
  const kind = getFileKind(file.type) ?? 'unknown';
  const previewUrl =
    kind === 'image' || kind === 'video'
      ? URL.createObjectURL(file)
      : null;
  return {
    id: crypto.randomUUID(),
    file,
    kind,
    previewUrl,
    validationError: validateFile(file),
    progress: { ...IDLE_PROGRESS, totalBytes: file.size },
    resultUrl: null,
    aborted: false,
  };
}

// ─── hook ─────────────────────────────────────────────────────────────────────

const MAX_CONCURRENT = 3;

export interface UseFileUploadReturn {
  items: UploadItem[];
  addFiles: (files: FileList | File[]) => void;
  startUpload: (id: string) => void;
  cancelUpload: (id: string) => void;
  removeItem: (id: string) => void;
  clearAll: () => void;
  updateStatus: (mediaId: string, status: MediaStatus) => void;
}

export function useFileUpload(_accessToken?: string | null): UseFileUploadReturn {
  const [items, setItems] = useState<UploadItem[]>([]);
  // abort controllers keyed by item id
  const abortRefs = useRef<Map<string, AbortController>>(new Map());
  const activeUploads = useRef(0);

  const patch = useCallback(
    (id: string, patch: Partial<UploadItem> | ((prev: UploadItem) => Partial<UploadItem>)) => {
      setItems((prev) =>
        prev.map((it) => {
          if (it.id !== id) return it;
          const update = typeof patch === 'function' ? patch(it) : patch;
          return { ...it, ...update };
        }),
      );
    },
    [],
  );

  const patchProgress = useCallback(
    (
      id: string,
      pp: Partial<UploadProgressState> | ((prev: UploadProgressState) => Partial<UploadProgressState>),
    ) => {
      setItems((prev) =>
        prev.map((it) => {
          if (it.id !== id) return it;
          const update = typeof pp === 'function' ? pp(it.progress) : pp;
          return { ...it, progress: { ...it.progress, ...update } };
        }),
      );
    },
    [],
  );

  const addFiles = useCallback((files: FileList | File[]) => {
    const arr = Array.from(files);
    setItems((prev) => {
      const next = arr.map(makeItem);
      return [...prev, ...next];
    });
  }, []);

  const startUpload = useCallback(
    async (id: string) => {
      if (activeUploads.current >= MAX_CONCURRENT) return;
      activeUploads.current += 1;

      const controller = new AbortController();
      abortRefs.current.set(id, controller);

      try {
        setItems((prev) => {
          const item = prev.find((it) => it.id === id);
          if (!item || item.validationError || item.aborted) return prev;
          return prev.map((it) =>
            it.id === id
              ? {
                  ...it,
                  progress: {
                    ...it.progress,
                    phase: 'CREATING_SESSION',
                    status: 'PENDING',
                  },
                }
              : it,
          );
        });

        // Read the item snapshot
        let snapshot: UploadItem | undefined;
        setItems((prev) => {
          snapshot = prev.find((it) => it.id === id);
          return prev;
        });
        if (!snapshot || snapshot.validationError || snapshot.aborted) return;

        const file = snapshot.file;
        const checksum = await sha256Hex(file);

        if (controller.signal.aborted) return;

        const session = await createUploadSession({
          fileName: file.name,
          contentType: file.type || 'application/octet-stream',
          size: file.size,
          checksum,
        });

        if (controller.signal.aborted) return;

        patchProgress(id, {
          mediaId: session.mediaId,
          uploadId: session.uploadId,
          phase: 'UPLOADING',
          uploadedBytes: 0,
          totalParts: session.totalParts,
          status: 'UPLOADING',
        });

        const completedParts: CompletedPart[] = await uploadFileParts(
          file,
          session,
          {
            onProgress: (uploadedBytes) => patchProgress(id, { uploadedBytes }),
            onPartComplete: (partNumber) =>
              patchProgress(id, (p) => ({
                uploadedParts: Math.max(p.uploadedParts, partNumber),
              })),
          },
        );

        if (controller.signal.aborted) return;

        patchProgress(id, { phase: 'COMPLETING' });
        const { mediaId } = await completeUpload(session.uploadId, completedParts);

        patchProgress(id, { mediaId, phase: 'DONE', status: 'UPLOADED' });
      } catch (err) {
        if (!controller.signal.aborted) {
          patchProgress(id, {
            phase: 'ERROR',
            error: err instanceof Error ? err.message : String(err),
          });
        }
      } finally {
        abortRefs.current.delete(id);
        activeUploads.current -= 1;
      }
    },
    [patchProgress],
  );

  const cancelUpload = useCallback(
    (id: string) => {
      const ctrl = abortRefs.current.get(id);
      ctrl?.abort();
      abortRefs.current.delete(id);
      patch(id, (it) => ({
        aborted: true,
        progress: { ...it.progress, phase: 'ERROR', error: 'تم الإلغاء' },
      }));
    },
    [patch],
  );

  const removeItem = useCallback(
    (id: string) => {
      abortRefs.current.get(id)?.abort();
      abortRefs.current.delete(id);
      setItems((prev) => {
        const item = prev.find((it) => it.id === id);
        if (item?.previewUrl) URL.revokeObjectURL(item.previewUrl);
        return prev.filter((it) => it.id !== id);
      });
    },
    [],
  );

  const clearAll = useCallback(() => {
    abortRefs.current.forEach((c) => c.abort());
    abortRefs.current.clear();
    setItems((prev) => {
      prev.forEach((it) => {
        if (it.previewUrl) URL.revokeObjectURL(it.previewUrl);
      });
      return [];
    });
    activeUploads.current = 0;
  }, []);

  const updateStatus = useCallback((mediaId: string, status: MediaStatus) => {
    setItems((prev) =>
      prev.map((it) =>
        it.progress.mediaId === mediaId
          ? { ...it, progress: { ...it.progress, status } }
          : it,
      ),
    );
  }, []);

  return { items, addFiles, startUpload, cancelUpload, removeItem, clearAll, updateStatus };
}
