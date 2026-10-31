import { useCallback, useRef, useState } from 'react';
import {
  ALLOWED_TYPES,
  formatBytes,
  useFileUpload,
  type UploadItem,
} from '../upload/useFileUpload';
import { UploadProgress } from './UploadProgress';

const ACCEPT = Object.keys(ALLOWED_TYPES).join(',');

function FileTypeIcon({ kind }: { kind: string }) {
  if (kind === 'image') return <span className="file-icon">🖼️</span>;
  if (kind === 'video') return <span className="file-icon">🎬</span>;
  if (kind === 'document') return <span className="file-icon">📄</span>;
  return <span className="file-icon">📁</span>;
}

function PreviewCell({ item }: { item: UploadItem }) {
  if (item.kind === 'image' && item.previewUrl) {
    return <img className="lab-preview-img" src={item.previewUrl} alt={item.file.name} />;
  }
  if (item.kind === 'video' && item.previewUrl) {
    return (
      <video className="lab-preview-video" src={item.previewUrl} controls muted preload="metadata" />
    );
  }
  if (item.kind === 'document') {
    return (
      <div className="lab-preview-doc">
        <span className="lab-doc-ext">{item.file.name.split('.').pop()?.toUpperCase()}</span>
        <span className="lab-doc-name">{item.file.name}</span>
      </div>
    );
  }
  return null;
}

function StatusBadge({ status }: { status: string | null }) {
  if (!status) return null;
  const cls = `status-badge status-badge-${status.toLowerCase()}`;
  return <span className={cls}>{status}</span>;
}

function ItemRow({
  item,
  onStart,
  onCancel,
  onRemove,
}: {
  item: UploadItem;
  onStart: () => void;
  onCancel: () => void;
  onRemove: () => void;
}) {
  const phase = item.progress.phase;
  const isActive = phase === 'UPLOADING' || phase === 'CREATING_SESSION' || phase === 'COMPLETING';
  const isDone = phase === 'DONE';
  const isIdle = phase === 'IDLE';
  const isError = phase === 'ERROR';

  return (
    <div className={`lab-item ${isError ? 'lab-item--error' : isDone ? 'lab-item--done' : ''}`}>
      <PreviewCell item={item} />

      <div className="lab-item-info">
        <div className="lab-item-name" title={item.file.name}>
          <FileTypeIcon kind={item.kind} />
          {item.file.name}
        </div>
        <div className="lab-item-meta">
          <span>{formatBytes(item.file.size)}</span>
          <span>{item.file.type || 'unknown'}</span>
          <StatusBadge status={item.progress.status} />
        </div>

        {item.validationError ? (
          <p className="error">{item.validationError}</p>
        ) : (
          <UploadProgress progress={item.progress} compact />
        )}
      </div>

      <div className="lab-item-actions">
        {isIdle && !item.validationError && (
          <button className="btn btn-primary btn-sm" onClick={onStart}>
            رفع
          </button>
        )}
        {isActive && (
          <button className="btn btn-ghost btn-sm" onClick={onCancel}>
            إلغاء
          </button>
        )}
        {(isDone || isError || item.validationError) && (
          <button className="btn btn-ghost btn-sm" onClick={onRemove}>
            حذف
          </button>
        )}
      </div>
    </div>
  );
}

export function FileUploadLab({ accessToken }: { accessToken: string | null }) {
  const { items, addFiles, startUpload, cancelUpload, removeItem, clearAll } =
    useFileUpload(accessToken);
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragging(false);
      if (e.dataTransfer.files.length > 0) {
        addFiles(e.dataTransfer.files);
      }
    },
    [addFiles],
  );

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setDragging(true);
  };
  const handleDragLeave = () => setDragging(false);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      addFiles(e.target.files);
      e.target.value = '';
    }
  };

  const pendingCount = items.filter((it) => it.progress.phase === 'IDLE' && !it.validationError).length;
  const startAll = () => {
    items
      .filter((it) => it.progress.phase === 'IDLE' && !it.validationError)
      .forEach((it) => startUpload(it.id));
  };

  return (
    <section className="card lab-panel">
      <div className="lab-header">
        <h2>مختبر رفع الملفات</h2>
        <div className="lab-header-actions">
          {pendingCount > 0 && (
            <button className="btn btn-primary btn-sm" onClick={startAll}>
              رفع الكل ({pendingCount})
            </button>
          )}
          {items.length > 0 && (
            <button className="btn btn-ghost btn-sm" onClick={clearAll}>
              مسح الكل
            </button>
          )}
        </div>
      </div>

      {/* Drop zone */}
      <div
        className={`drop-zone ${dragging ? 'drop-zone--active' : ''}`}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onClick={() => inputRef.current?.click()}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => e.key === 'Enter' && inputRef.current?.click()}
      >
        <input
          ref={inputRef}
          type="file"
          multiple
          accept={ACCEPT}
          style={{ display: 'none' }}
          onChange={handleInputChange}
        />
        <div className="drop-zone-content">
          <span className="drop-icon">☁️</span>
          <p>اسحب وأفلت الملفات هنا أو انقر للاختيار</p>
          <p className="muted small">
            صور · فيديو (حتى 50 GB) · PDF / مستندات (حتى 500 MB)
          </p>
        </div>
      </div>

      {/* Queue */}
      {items.length > 0 && (
        <div className="lab-queue">
          {items.map((item) => (
            <ItemRow
              key={item.id}
              item={item}
              onStart={() => startUpload(item.id)}
              onCancel={() => cancelUpload(item.id)}
              onRemove={() => removeItem(item.id)}
            />
          ))}
        </div>
      )}
    </section>
  );
}
