import type { MediaStatus } from '../upload/types';

const LABELS: Partial<Record<MediaStatus, string>> = {
  PENDING: 'معلق',
  UPLOADING: 'رفع',
  UPLOADED: 'تم الرفع',
  SCANNING: 'فحص أمني',
  PROCESSING: 'معالجة',
  READY: 'جاهز',
  FAILED: 'فشل',
  QUARANTINED: 'معزول',
  DELETING: 'حذف',
  DELETED: 'محذوف',
  ABORTED: 'ملغي',
};

export function StatusBadge({ status }: { status: MediaStatus | null }) {
  if (!status) return null;
  const className = `status-badge status-badge-${status.toLowerCase()}`;
  return (
    <span className={className} title={status}>
      {LABELS[status] ?? status}
    </span>
  );
}