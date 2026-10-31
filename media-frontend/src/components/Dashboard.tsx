import { useState } from 'react';
import { useAuth } from '../auth/AuthContext';
import { useUploadAvatar } from '../upload/useUploadAvatar';
import { AvatarPicker, useAvatarPicker } from './AvatarPicker';
import { MediaLabPro } from './MediaLabPro';
import { RealtimePanel } from './RealtimePanel';
import { UploadProgress } from './UploadProgress';
import { TestingDashboard } from './TestingDashboard';

export function Dashboard({ onLogout }: { onLogout: () => void }) {
  const { user, accessToken, refreshProfile } = useAuth();
  const { progress, uploadAvatar } = useUploadAvatar(accessToken);
  const avatar = useAvatarPicker();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const startUpload = async () => {
    if (!avatar.file) return;
    setBusy(true);
    setError(null);
    try {
      await uploadAvatar(avatar.file);
      await refreshProfile().catch(() => undefined);
      avatar.clear();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="dashboard-full">
      {/* ── Top bar ──────────────────────────────────────────── */}
      <header className="topbar topbar-full">
        <div className="topbar-brand">
          <span className="topbar-logo">🚀</span>
          <h1>Realtime Delivery — Media Lab</h1>
        </div>
        <div className="topbar-user">
          {user?.imageUrl && (
            <img className="topbar-avatar" src={user.imageUrl} alt="avatar" />
          )}
          <span className="muted">{user?.firstName} {user?.lastName}</span>
          <button className="btn btn-ghost btn-sm" onClick={onLogout}>
            تسجيل الخروج
          </button>
        </div>
      </header>

      {/* ── 3-column grid ────────────────────────────────────── */}
      <div className="dashboard-3col">

        {/* Column 1: Profile + Avatar Upload */}
        <div className="dash-col">
          <section className="card profile-card">
            <h2>الملف الشخصي</h2>
            <div className="profile-avatar-wrap">
              {user?.imageUrl ? (
                <img className="avatar-large" src={user.imageUrl} alt="avatar" />
              ) : (
                <div className="avatar-placeholder avatar-large">لا توجد صورة</div>
              )}
            </div>
            <p className="profile-name">{user?.firstName} {user?.lastName}</p>
            <p className="muted">{user?.email}</p>
            <p className="muted">الدور: <strong>{user?.role}</strong></p>
            {user?.imageUrl && (
              <p className="muted small code-wrap">
                <span className="code">{user.imageUrl}</span>
              </p>
            )}
          </section>

          <section className="card">
            <h2>رفع صورة البروفايل</h2>
            <AvatarPicker
              previewUrl={avatar.previewUrl}
              error={avatar.error}
              onPick={avatar.pick}
              onClear={avatar.clear}
              disabled={busy || progress.phase === 'UPLOADING'}
            />
            <button
              className="btn btn-primary"
              style={{ width: '100%', marginTop: 8 }}
              onClick={startUpload}
              disabled={!avatar.file || busy || progress.phase === 'UPLOADING'}
            >
              {busy ? 'جارٍ الرفع…' : 'رفع الصورة'}
            </button>
            {error && <p className="error">{error}</p>}
            <UploadProgress progress={progress} />
          </section>
        </div>

        {/* Column 2: Advanced Media Lab Pro */}
        <div className="dash-col">
          <MediaLabPro accessToken={accessToken} />
        </div>

        {/* Column 3: Realtime Events */}
        <div className="dash-col">
          <RealtimePanel accessToken={accessToken} />
        </div>
        
        {/* Column 4: Testing Dashboard */}
        <div className="dash-col">
          <TestingDashboard />
        </div>
      </div>
    </div>
  );
}