import { useState } from 'react';
import { useAuth } from '../auth/AuthContext';
import { useUploadAvatar } from '../upload/useUploadAvatar';
import { AvatarPicker, useAvatarPicker } from './AvatarPicker';
import { UploadProgress } from './UploadProgress';

function AuthPanel({
  title,
  busy,
  children,
  footer,
  onSubmit,
}: {
  title: string;
  busy: boolean;
  children: React.ReactNode;
  footer: React.ReactNode;
  onSubmit: (e: React.FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <form className="card form" onSubmit={onSubmit}>
      <h2>{title}</h2>
      {children}
      <button className="btn btn-primary" type="submit" disabled={busy}>
        {busy ? 'جاري التنفيذ…' : title}
      </button>
      <div className="form-footer">{footer}</div>
    </form>
  );
}

export function AuthFlow({ onComplete }: { onComplete: () => void }) {
  const { user, accessToken, registerAndLogin, loginWithEmail, refreshProfile, loading } = useAuth();
  const { progress, uploadAvatar, reset } = useUploadAvatar(accessToken);
  const avatar = useAvatarPicker();
  const [mode, setMode] = useState<'register' | 'login'>('register');

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  // Generate a unique Egyptian mobile number per mount so repeat registrations don't clash
  // (format: +20 + 10-digit national number starting with 1).
  const [phoneNumber, setPhoneNumber] = useState(() => {
    const rand = Math.floor(Math.random() * 900000000) + 100000000;
    return `+201${rand}`;
  });
  const [error, setError] = useState<string | null>(null);

  const busy = loading || progress.phase === 'UPLOADING' || progress.phase === 'COMPLETING' || progress.phase === 'LINKING_AVATAR' || progress.phase === 'CREATING_SESSION';

  const submit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError(null);
    reset();
    try {
      if (mode === 'register') {
        await registerAndLogin({ email, password, firstName, lastName, phoneNumber });
        if (avatar.file) {
          await uploadAvatar(avatar.file);
        }
      } else {
        await loginWithEmail(email, password);
      }
      await refreshProfile().catch(() => undefined);
      onComplete();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="auth-layout">
      <AuthPanel
        title={mode === 'register' ? 'تسجيل حساب جديد' : 'تسجيل الدخول'}
        busy={busy}
        footer={
          mode === 'register' ? (
            <>
              لديك حساب؟{' '}
              <button type="button" className="link" onClick={() => setMode('login')}>
                تسجيل الدخول
              </button>
            </>
          ) : (
            <>
              ليس لديك حساب؟{' '}
              <button type="button" className="link" onClick={() => setMode('register')}>
                إنشاء حساب
              </button>
            </>
          )
        }
        onSubmit={submit}
      >
        {mode === 'register' && (
          <>
            <div className="field-row">
              <input
                placeholder="الاسم الأول"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                required
              />
              <input
                placeholder="الاسم الأخير"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                required
              />
            </div>
            <input
              placeholder="رقم الهاتف (EG: +201...)"
              value={phoneNumber}
              onChange={(e) => setPhoneNumber(e.target.value)}
              required
            />
            <AvatarPicker
              previewUrl={avatar.previewUrl}
              error={avatar.error}
              onPick={avatar.pick}
              onClear={avatar.clear}
              disabled={busy}
            />
          </>
        )}
        <input
          type="email"
          placeholder="البريد الإلكتروني"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
        />
        <input
          type="password"
          placeholder="كلمة المرور (8+ أحرف)"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          minLength={8}
          required
        />
        {avatar.error && <p className="error">{avatar.error}</p>}
        {error && <p className="error">{error}</p>}
        {user && mode === 'register' && (
          <UploadProgress progress={progress} compact />
        )}
      </AuthPanel>
    </div>
  );
}