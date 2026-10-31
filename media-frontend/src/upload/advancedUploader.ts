import { uploadStorage } from './indexedDB';
import { createUploadSession, completeUpload, getUploadStatus, abortUpload } from '../api/operations';
import { sha256Hex } from './presignedUpload';

export interface PartUploadStatus {
  partNumber: number;
  uploadedBytes: number;
  totalBytes: number;
  status: 'IDLE' | 'UPLOADING' | 'COMPLETED' | 'FAILED';
}

export interface UploadProgressInfo {
  phase: 'IDLE' | 'HASHING' | 'CREATING_SESSION' | 'UPLOADING' | 'COMPLETING' | 'DONE' | 'PAUSED' | 'ERROR';
  uploadedBytes: number;
  totalBytes: number;
  speedBytesPerSec: number;
  parts: PartUploadStatus[];
  error?: string;
}

export type ProgressCallback = (info: UploadProgressInfo) => void;

const CHUNK_SIZE = 5 * 1024 * 1024; // 5 MB
const CONCURRENCY = 4;
const MAX_RETRIES = 5;
const BASE_BACKOFF = 1000;

export class AdvancedUploader {
  private file: File;
  private fileId: string;
  private onProgress: ProgressCallback;
  private abortController: AbortController | null = null;
  private isPaused = false;
  private isOffline = false;

  // Session state
  private mediaId: string | null = null;
  private uploadId: string | null = null;
  private s3UploadId: string | null = null;
  private presignedParts: { partNumber: number; presignedUrl: string }[] = [];
  private totalParts = 0;
  private partSize = CHUNK_SIZE;
  private completedPartsMap = new Map<number, string>(); // partNumber -> ETag

  // Progress metrics
  private partStatuses: PartUploadStatus[] = [];
  private lastProgressTime = 0;
  private lastUploadedBytes = 0;
  private currentSpeed = 0;

  constructor(file: File, onProgress: ProgressCallback) {
    this.file = file;
    this.fileId = `${file.name}-${file.size}-${file.lastModified}`;
    this.onProgress = onProgress;
    this.setupNetworkListeners();
  }

  private setupNetworkListeners() {
    window.addEventListener('online', this.handleOnline);
    window.addEventListener('offline', this.handleOffline);
    this.isOffline = !navigator.onLine;
  }

  public destroy() {
    window.removeEventListener('online', this.handleOnline);
    window.removeEventListener('offline', this.handleOffline);
    this.cancel();
  }

  private handleOnline = () => {
    this.isOffline = false;
    if (this.isPaused && !this.abortController) {
      console.log('Network restored. Resuming upload...');
      this.resume();
    }
  };

  private handleOffline = () => {
    this.isOffline = true;
    console.log('Network disconnected. Pausing upload...');
    this.pauseInternal('تم قطع الاتصال بالإنترنت. جاري الانتظار لاستعادة الشبكة...');
  };

  private emit(phase: UploadProgressInfo['phase'], error?: string) {
    let totalUploaded = 0;
    for (const p of this.partStatuses) {
      totalUploaded += p.uploadedBytes;
    }

    const now = Date.now();
    const timeDiff = (now - this.lastProgressTime) / 1000;
    if (timeDiff >= 1) {
      const bytesDiff = totalUploaded - this.lastUploadedBytes;
      this.currentSpeed = bytesDiff / timeDiff;
      this.lastUploadedBytes = totalUploaded;
      this.lastProgressTime = now;
    }

    this.onProgress({
      phase,
      uploadedBytes: totalUploaded,
      totalBytes: this.file.size,
      speedBytesPerSec: this.currentSpeed,
      parts: [...this.partStatuses],
      error,
    });
  }

  private async initializeSession(): Promise<boolean> {
    this.emit('HASHING');
    const checksum = await sha256Hex(this.file);

    // Try loading existing session
    const saved = await uploadStorage.getSession(this.fileId);
    if (saved) {
      // Validate session with backend to get actual status
      try {
        const backend = await getUploadStatus(saved.uploadId);
        this.mediaId = saved.mediaId;
        this.uploadId = saved.uploadId;
        this.s3UploadId = saved.s3UploadId;
        this.presignedParts = saved.completedParts.map(p => ({
          partNumber: p.partNumber,
          presignedUrl: '', // URLs might have expired, but we can re-request or bypass if completed
        }));
        this.totalParts = saved.totalParts;
        this.partSize = saved.partSize;

        // If backend tells us expired or already completed, handle it
        if (backend.status === 'COMPLETED') {
          this.emit('DONE');
          return false;
        }

        // Sync completed parts from backend
        // Assume backend returns missingParts list
        const missing = new Set(backend.missingParts);
        for (const p of saved.completedParts) {
          if (!missing.has(p.partNumber)) {
            this.completedPartsMap.set(p.partNumber, p.eTag);
          }
        }
        
        // S3 upload status might have updated URLs. Let's request a new session or keep existing
        // If presignedUrls are expired, we might need a way to refresh them.
        // For simplicity, S3 multipart URLs might be generated. In the schema, `createUploadSession`
        // acts as a creator. If we already have one, the backend `createUploadSession` is idempotent
        // if idempotencyKey is used. But for the test page, we can just request again.
        // For now, let's keep the client cached URLs unless they fail.
        this.presignedParts = saved.presignedParts;
      } catch (err) {
        // Cached session expired on backend, clear it
        await uploadStorage.deleteSession(this.fileId);
      }
    }

    if (!this.uploadId) {
      this.emit('CREATING_SESSION');
      const session = await createUploadSession({
        fileName: this.file.name,
        contentType: this.file.type || 'application/octet-stream',
        size: this.file.size,
        checksum,
      });

      this.mediaId = session.mediaId;
      this.uploadId = session.uploadId;
      this.s3UploadId = session.s3UploadId;
      this.presignedParts = session.presignedParts || [];
      this.totalParts = session.totalParts;
      this.partSize = session.partSize;

      await this.saveSessionToStorage();
    }

    // Initialize statuses
    this.partStatuses = Array.from({ length: this.totalParts }, (_, i) => {
      const partNum = i + 1;
      const size = partNum === this.totalParts ? this.file.size - (i * this.partSize) : this.partSize;
      const isDone = this.completedPartsMap.has(partNum);
      return {
        partNumber: partNum,
        uploadedBytes: isDone ? size : 0,
        totalBytes: size,
        status: isDone ? 'COMPLETED' : 'IDLE',
      };
    });

    return true;
  }

  private async saveSessionToStorage() {
    if (!this.mediaId || !this.uploadId || !this.s3UploadId) return;
    const completed = Array.from(this.completedPartsMap.entries()).map(([partNumber, eTag]) => ({
      partNumber,
      eTag,
    }));
    await uploadStorage.saveSession({
      fileId: this.fileId,
      mediaId: this.mediaId,
      uploadId: this.uploadId,
      s3UploadId: this.s3UploadId,
      totalParts: this.totalParts,
      partSize: this.partSize,
      fileName: this.file.name,
      fileSize: this.file.size,
      completedParts: completed,
      presignedParts: this.presignedParts,
      expiresAt: Date.now() + 24 * 60 * 60 * 1000,
    });
  }

  public async start() {
    this.isPaused = false;
    this.lastProgressTime = Date.now();
    this.lastUploadedBytes = 0;

    try {
      const active = await this.initializeSession();
      if (!active) return;

      this.abortController = new AbortController();
      this.emit('UPLOADING');
      await this.runUploadPool();
    } catch (err) {
      if (!this.isPaused) {
        this.emit('ERROR', err instanceof Error ? err.message : String(err));
      }
    }
  }

  private async runUploadPool() {
    const queue = Array.from({ length: this.totalParts }, (_, i) => i + 1)
      .filter((partNum) => !this.completedPartsMap.has(partNum));

    let index = 0;
    const workers: Promise<void>[] = [];

    const worker = async () => {
      while (index < queue.length && !this.isPaused && !this.isOffline) {
        const partNum = queue[index];
        index++;
        await this.uploadPartWithRetry(partNum);
      }
    };

    const workerCount = Math.min(CONCURRENCY, queue.length);
    for (let i = 0; i < workerCount; i++) {
      workers.push(worker());
    }

    await Promise.all(workers);

    if (this.isPaused || this.isOffline) {
      return;
    }

    if (this.completedPartsMap.size === this.totalParts) {
      await this.complete();
    } else {
      throw new Error('لم يتم رفع جميع الأجزاء بنجاح.');
    }
  }

  private async uploadPartWithRetry(partNum: number) {
    const partInfo = this.presignedParts.find(p => p.partNumber === partNum);
    if (!partInfo) throw new Error(`لم يتم العثور على عنوان رفع للجزء ${partNum}`);

    const start = (partNum - 1) * this.partSize;
    const end = Math.min(start + this.partSize, this.file.size);
    const blob = this.file.slice(start, end);

    let lastError: any = null;
    for (let attempt = 0; attempt < MAX_RETRIES; attempt++) {
      if (this.isPaused || this.isOffline) return;

      try {
        this.updatePartStatus(partNum, 'UPLOADING', 0);
        
        const signal = this.abortController?.signal;
        
        // Simple XMLHttpRequest for accurate upload progress
        const eTag = await this.uploadPartXHR(partInfo.presignedUrl, blob, (uploaded) => {
          this.updatePartStatus(partNum, 'UPLOADING', uploaded);
          this.emit('UPLOADING');
        }, signal);

        this.completedPartsMap.set(partNum, eTag);
        this.updatePartStatus(partNum, 'COMPLETED', blob.size);
        await this.saveSessionToStorage();
        return;
      } catch (err) {
        lastError = err;
        this.updatePartStatus(partNum, 'FAILED', 0);
        this.emit('UPLOADING');

        if (attempt < MAX_RETRIES - 1) {
          const delay = BASE_BACKOFF * Math.pow(2, attempt) + Math.random() * 1000;
          await new Promise((resolve) => setTimeout(resolve, delay));
        }
      }
    }
    throw lastError || new Error(`فشل رفع الجزء ${partNum}`);
  }

  private uploadPartXHR(url: string, blob: Blob, onProgress: (bytes: number) => void, signal?: AbortSignal): Promise<string> {
    return new Promise((resolve, reject) => {
      const uploadUrl = url.replace('http://localstack-srv:4566', `http://${window.location.hostname}:4566`);
      const xhr = new XMLHttpRequest();
      xhr.open('PUT', uploadUrl);
      xhr.setRequestHeader('Content-Type', 'application/octet-stream');

      xhr.upload.onprogress = (ev) => {
        if (ev.lengthComputable) {
          onProgress(ev.loaded);
        }
      };

      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          const eTag = xhr.getResponseHeader('ETag');
          if (eTag) {
            resolve(eTag.replace(/"/g, ''));
          } else {
            reject(new Error('Missing ETag header'));
          }
        } else {
          reject(new Error(`HTTP Error: ${xhr.status}`));
        }
      };

      xhr.onerror = () => reject(new Error('Network error'));
      xhr.onabort = () => reject(new Error('Aborted'));

      if (signal) {
        signal.addEventListener('abort', () => xhr.abort());
      }

      xhr.send(blob);
    });
  }

  private updatePartStatus(partNum: number, status: PartUploadStatus['status'], uploaded: number) {
    const idx = partNum - 1;
    if (this.partStatuses[idx]) {
      this.partStatuses[idx].status = status;
      this.partStatuses[idx].uploadedBytes = uploaded;
    }
  }

  private async complete() {
    this.emit('COMPLETING');
    if (!this.uploadId) return;

    const parts = Array.from(this.completedPartsMap.entries()).map(([partNumber, eTag]) => ({
      partNumber,
      eTag,
    }));

    await completeUpload(this.uploadId, parts);
    await uploadStorage.deleteSession(this.fileId);
    this.emit('DONE');
  }

  public pause() {
    this.pauseInternal('تم إيقاف الرفع مؤقتاً.');
  }

  private pauseInternal(reason: string) {
    this.isPaused = true;
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
    this.emit('PAUSED', reason);
  }

  public async resume() {
    await this.start();
  }

  public async cancel() {
    this.isPaused = true;
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
    
    if (this.uploadId) {
      try {
        await abortUpload(this.uploadId);
      } catch (e) {
        console.error('Failed to abort upload session on backend:', e);
      }
      await uploadStorage.deleteSession(this.fileId);
    }

    this.emit('IDLE');
  }

  public getMediaId() {
    return this.mediaId;
  }
}
