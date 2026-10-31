export interface ResumableDownloadOptions {
  url: string;
  fileName: string;
  contentType?: string;
  onProgress?: (receivedBytes: number, totalBytes: number | null) => void;
  signal?: AbortSignal;
}

const MAX_RETRIES = 4;
const BASE_BACKOFF_MS = 500;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

function downloadName(name: string): string {
  return name.replace(/[\\/\0]/g, '_').trim() || 'download';
}

export async function downloadWithResume(options: ResumableDownloadOptions): Promise<void> {
  const chunks: Uint8Array[] = [];
  let received = 0;
  let total: number | null = null;
  let attempt = 0;

  while (true) {
    const headers: Record<string, string> = {};
    if (received > 0) headers.Range = `bytes=${received}-`;

    try {
      const response = await fetch(options.url, {
        headers,
        signal: options.signal,
        cache: 'no-store',
      });
      if (!response.ok) throw new Error(`download failed: HTTP ${response.status}`);

      const isPartial = response.status === 206;
      if (received > 0 && !isPartial) {
        // The origin ignored Range. Restart once rather than corrupting output.
        chunks.length = 0;
        received = 0;
      }

      const contentLength = Number(response.headers.get('content-length'));
      const contentRange = response.headers.get('content-range');
      const rangeTotal = contentRange?.match(/\/(\d+)$/)?.[1];
      total = rangeTotal ? Number(rangeTotal) : Number.isFinite(contentLength) ? received + contentLength : total;

      if (!response.body) throw new Error('download response has no body');
      const reader = response.body.getReader();
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        if (value) {
          chunks.push(value);
          received += value.byteLength;
          options.onProgress?.(received, total);
        }
      }

      if (total !== null && received < total) {
        throw new Error(`download ended early at ${received}/${total} bytes`);
      }
      break;
    } catch (error) {
      if (options.signal?.aborted) throw error;
      if (attempt >= MAX_RETRIES) throw error;
      await sleep(BASE_BACKOFF_MS * 2 ** attempt + Math.random() * 250);
      attempt += 1;
    }
  }

  const blob = new Blob(chunks as BlobPart[], { type: options.contentType || 'application/octet-stream' });
  const objectUrl = URL.createObjectURL(blob);
  try {
    const anchor = document.createElement('a');
    anchor.href = objectUrl;
    anchor.download = downloadName(options.fileName);
    anchor.rel = 'noopener';
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
  } finally {
    setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
  }
}
