import type { CompletedPart, PresignedPart, UploadSession } from './types';

export interface UploadCallbacks {
  onProgress: (uploadedBytes: number) => void;
  onPartComplete: (partNumber: number) => void;
}

const CONCURRENCY = 4;
const MAX_PART_RETRIES = 3;
const BASE_BACKOFF_MS = 1000;
const MAX_BACKOFF_MS = 8000;

function backoffMs(attempt: number): number {
  const exp = Math.min(MAX_BACKOFF_MS, BASE_BACKOFF_MS * 2 ** attempt);
  return exp / 2 + Math.random() * (exp / 2); // + jitter
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function uploadPart(
  file: File,
  part: PresignedPart,
  partSize: number,
  partNumber: number,
): Promise<string> {
  const start = (partNumber - 1) * partSize;
  const end = Math.min(start + partSize, file.size);
  const blob = file.slice(start, end);

  let lastErr: Error | null = null;
  for (let attempt = 0; attempt < MAX_PART_RETRIES; attempt++) {
    try {
      // Format URL so localstack-srv internal k8s host is reachable from the browser
      const uploadUrl = part.presignedUrl.replace('http://localstack-srv:4566', `http://${window.location.hostname}:4566`);
      const resp = await fetch(uploadUrl, {
        method: 'PUT',
        body: blob,
        headers: { 'Content-Type': 'application/octet-stream' },
      });
      if (!resp.ok) {
        throw new Error(`part ${partNumber} PUT failed: HTTP ${resp.status}`);
      }
      const eTag = resp.headers.get('ETag');
      if (!eTag) {
        throw new Error(
          `part ${partNumber} response missing ETag header (S3 CORS must expose ETag)`,
        );
      }
      return eTag;
    } catch (err) {
      lastErr = err instanceof Error ? err : new Error(String(err));
      if (attempt < MAX_PART_RETRIES - 1) {
        await sleep(backoffMs(attempt));
      }
    }
  }
  throw lastErr || new Error(`part ${partNumber} failed`);
}

export async function uploadFileParts(
  file: File,
  session: UploadSession,
  callbacks: UploadCallbacks,
): Promise<CompletedPart[]> {
  const { presignedParts, partSize, totalParts } = session;
  if (!presignedParts || presignedParts.length === 0) {
    throw new Error('no presigned parts returned for upload session');
  }

  const parts = [...presignedParts].sort((a, b) => a.partNumber - b.partNumber);
  const results: (CompletedPart | null)[] = new Array(totalParts).fill(null);
  let cursor = 0;
  let uploadedBytes = 0;

  const worker = async () => {
    while (cursor < parts.length) {
      const part = parts[cursor];
      cursor += 1;
      const eTag = await uploadPart(file, part, partSize, part.partNumber);
      results[part.partNumber - 1] = { partNumber: part.partNumber, eTag };
      uploadedBytes += Math.min(partSize, file.size - (part.partNumber - 1) * partSize);
      callbacks.onProgress(uploadedBytes);
      callbacks.onPartComplete(part.partNumber);
    }
  };

  await Promise.all(Array.from({ length: Math.min(CONCURRENCY, parts.length) }, worker));

  const completed = results.filter((r): r is CompletedPart => r !== null);
  if (completed.length !== totalParts) {
    throw new Error(`upload incomplete: ${completed.length}/${totalParts} parts uploaded`);
  }
  return completed;
}

export function validateClientFile(file: File): string | null {
  const MAX_IMAGE_BYTES = 10 * 1024 * 1024;
  const ALLOWED = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];
  if (!ALLOWED.includes(file.type)) {
    return `unsupported type "${file.type}" (client validation)`;
  }
  if (file.size > MAX_IMAGE_BYTES) {
    return 'file exceeds 10 MB (client validation)';
  }
  return null;
}

export async function sha256Hex(blob: Blob): Promise<string> {
  const buffer = await blob.arrayBuffer();
  const digest = await crypto.subtle.digest('SHA-256', buffer);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}