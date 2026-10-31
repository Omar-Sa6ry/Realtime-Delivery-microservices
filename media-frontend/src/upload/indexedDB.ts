export interface StoredSession {
  fileId: string; // unique ID generated locally for the file instance (e.g. name + size + lastModified)
  mediaId: string;
  uploadId: string;
  s3UploadId: string;
  totalParts: number;
  partSize: number;
  fileName: string;
  fileSize: number;
  completedParts: { partNumber: number; eTag: string }[];
  presignedParts: { partNumber: number; presignedUrl: string }[];
  expiresAt: number;
}

class UploadStorage {
  private dbName = 'MediaUploadDB';
  private storeName = 'sessions';
  private db: IDBDatabase | null = null;

  private initDB(): Promise<IDBDatabase> {
    if (this.db) return Promise.resolve(this.db);
    return new Promise((resolve, reject) => {
      const request = indexedDB.open(this.dbName, 1);
      request.onupgradeneeded = () => {
        const db = request.result;
        if (!db.objectStoreNames.contains(this.storeName)) {
          db.createObjectStore(this.storeName, { keyPath: 'fileId' });
        }
      };
      request.onsuccess = () => {
        this.db = request.result;
        resolve(this.db);
      };
      request.onerror = () => reject(request.error);
    });
  }

  async saveSession(session: StoredSession): Promise<void> {
    const db = await this.initDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(this.storeName, 'readwrite');
      const store = tx.objectStore(this.storeName);
      const req = store.put(session);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }

  async getSession(fileId: string): Promise<StoredSession | null> {
    const db = await this.initDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(this.storeName, 'readonly');
      const store = tx.objectStore(this.storeName);
      const req = store.get(fileId);
      req.onsuccess = () => {
        const res = req.result;
        if (res && res.expiresAt > Date.now()) {
          resolve(res);
        } else {
          if (res) this.deleteSession(fileId); // Cleanup expired
          resolve(null);
        }
      };
      req.onerror = () => reject(req.error);
    });
  }

  async deleteSession(fileId: string): Promise<void> {
    const db = await this.initDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(this.storeName, 'readwrite');
      const store = tx.objectStore(this.storeName);
      const req = store.delete(fileId);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }
}

export const uploadStorage = new UploadStorage();
