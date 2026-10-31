import { useState } from 'react';

export interface AvatarPickerValue {
  file: File | null;
  previewUrl: string | null;
  error: string | null;
}

export function useAvatarPicker(maxBytes = 10 * 1024 * 1024) {
  const [value, setValue] = useState<AvatarPickerValue>({
    file: null,
    previewUrl: null,
    error: null,
  });

  const pick = (file: File | undefined) => {
    if (!file) {
      setValue({ file: null, previewUrl: null, error: null });
      return;
    }
    const allowed = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];
    if (!allowed.includes(file.type)) {
      setValue({ file: null, previewUrl: null, error: `نوع الملف غير مدعوم: ${file.type}` });
      return;
    }
    if (file.size > maxBytes) {
      setValue({ file: null, previewUrl: null, error: 'الملف أكبر من 10 ميجابايت' });
      return;
    }
    if (value.previewUrl) URL.revokeObjectURL(value.previewUrl);
    const previewUrl = URL.createObjectURL(file);
    setValue({ file, previewUrl, error: null });
  };

  const clear = () => {
    if (value.previewUrl) URL.revokeObjectURL(value.previewUrl);
    setValue({ file: null, previewUrl: null, error: null });
  };

  return { ...value, pick, clear };
}

export function AvatarPicker({
  previewUrl,
  error,
  onPick,
  onClear,
  disabled,
}: {
  previewUrl: string | null;
  error: string | null;
  onPick: (file: File) => void;
  onClear: () => void;
  disabled?: boolean;
}) {
  return (
    <div className="avatar-picker">
      {previewUrl ? (
        <img className="avatar-preview" src={previewUrl} alt="avatar preview" />
      ) : (
        <div className="avatar-placeholder">صورة البروفايل</div>
      )}
      <label className="btn btn-ghost">
        اختر صورة
        <input
          type="file"
          accept="image/jpeg,image/png,image/webp,image/gif"
          disabled={disabled}
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) onPick(f);
          }}
          hidden
        />
      </label>
      {previewUrl && (
        <button type="button" className="btn btn-ghost" onClick={onClear} disabled={disabled}>
          إزالة
        </button>
      )}
      {error && <p className="error">{error}</p>}
    </div>
  );
}