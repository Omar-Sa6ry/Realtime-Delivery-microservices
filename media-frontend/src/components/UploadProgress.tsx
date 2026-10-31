import type { UploadProgressState } from '../upload/types';

function phaseLabel(phase: UploadProgressState['phase']): string {
  switch (phase) {
    case 'CREATING_SESSION':
      return 'إنشاء جلسة الرفع…';
    case 'UPLOADING':
      return 'الرفع إلي S3…';
    case 'COMPLETING':
      return 'إنهاء الرفع…';
    case 'LINKING_AVATAR':
      return 'ربط الصورة بالملف الشخصي (gRPC)…';
    case 'DONE':
      return 'تم';
    case 'ERROR':
      return 'فشل';
    default:
      return 'في انتظار…';
  }
}

export function UploadProgress({
  progress,
  compact,
}: {
  progress: UploadProgressState;
  compact?: boolean;
}) {
  if (progress.phase === 'IDLE') return null;

  const percent =
    progress.totalBytes > 0
      ? Math.round((progress.uploadedBytes / progress.totalBytes) * 100)
      : 0;

  return (
    <div className={`upload-progress${compact ? ' compact' : ''}`}>
      <div className="progress-head">
        <span>{phaseLabel(progress.phase)}</span>
        <span>
          {progress.status && (
            <span>
              {' '}
              · <span className="status-badge status-badge-pending">{progress.status}</span>
            </span>
          )}
        </span>
      </div>

      {progress.phase === 'UPLOADING' && (
        <>
          <div className="progress-track">
            <div className="progress-fill" style={{ width: `${percent}%` }} />
          </div>
          <div className="progress-meta">
            <span>
              {progress.uploadedParts}/{progress.totalParts} parts
            </span>
            <span>{percent}%</span>
          </div>
        </>
      )}

      {progress.error && <p className="error">{progress.error}</p>}
    </div>
  );
}