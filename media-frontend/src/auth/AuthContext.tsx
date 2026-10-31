import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { fetchMyProfile, login, register, type RegisterArgs } from '../api/operations';
import type { User } from '../upload/types';

interface AuthContextValue {
  user: User | null;
  accessToken: string | null;
  loading: boolean;
  registerAndLogin: (input: RegisterArgs) => Promise<void>;
  loginWithEmail: (email: string, password: string) => Promise<void>;
  refreshProfile: () => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function readStoredUser(): User | null {
  try {
    const raw = localStorage.getItem('user');
    return raw ? (JSON.parse(raw) as User) : null;
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(readStoredUser);
  const [accessToken, setAccessToken] = useState<string | null>(
    localStorage.getItem('accessToken'),
  );
  const [loading, setLoading] = useState(false);

  const applyAuth = useCallback((payload: { accessToken: string; user: User }) => {
    localStorage.setItem('accessToken', payload.accessToken);
    localStorage.setItem('user', JSON.stringify(payload.user));
    setAccessToken(payload.accessToken);
    setUser(payload.user);
  }, []);

  const registerAndLogin = useCallback(
    async (input: RegisterArgs) => {
      setLoading(true);
      try {
        const payload = await register(input);
        applyAuth(payload);
      } finally {
        setLoading(false);
      }
    },
    [applyAuth],
  );

  const loginWithEmail = useCallback(
    async (email: string, password: string) => {
      setLoading(true);
      try {
        const payload = await login({ email, password });
        applyAuth(payload);
      } finally {
        setLoading(false);
      }
    },
    [applyAuth],
  );

  const refreshProfile = useCallback(async () => {
    if (!accessToken) return;
    const profile = await fetchMyProfile();
    localStorage.setItem('user', JSON.stringify(profile));
    setUser(profile);
  }, [accessToken]);

  const logout = useCallback(() => {
    localStorage.removeItem('accessToken');
    localStorage.removeItem('user');
    setAccessToken(null);
    setUser(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      accessToken,
      loading,
      registerAndLogin,
      loginWithEmail,
      refreshProfile,
      logout,
    }),
    [user, accessToken, loading, registerAndLogin, loginWithEmail, refreshProfile, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within <AuthProvider>');
  return ctx;
}