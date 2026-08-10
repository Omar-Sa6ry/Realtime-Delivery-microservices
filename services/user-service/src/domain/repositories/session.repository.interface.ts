export interface UserSessionData {
  userId: string;
  email: string;
  role: string;
  sessionId: string;
  createdAt: Date;
}

export interface ISessionRepository {
  createSession(userId: string, sessionId: string, data: UserSessionData, ttlSeconds: number): Promise<void>;
  getSession(userId: string, sessionId: string): Promise<UserSessionData | null>;
  revokeSession(userId: string, sessionId: string): Promise<void>;
  revokeAllUserSessions(userId: string): Promise<void>;
  storeRefreshToken(refreshTokenHash: string, userId: string, ttlSeconds: number): Promise<void>;
  getUserIdByRefreshToken(refreshTokenHash: string): Promise<string | null>;
  revokeRefreshToken(refreshTokenHash: string): Promise<void>;
}

export const ISESSION_REPOSITORY = Symbol('ISessionRepository');
