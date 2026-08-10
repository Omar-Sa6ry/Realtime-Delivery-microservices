export interface TokenPayload {
  userId: string;
  email: string;
  role: string;
  permissions: string[];
  sessionId?: string;
}

export interface TokenResult {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export interface ITokenProvider {
  generateTokens(payload: TokenPayload): Promise<TokenResult>;
  verifyAccessToken(token: string): Promise<TokenPayload | null>;
  verifyRefreshToken(token: string): Promise<TokenPayload | null>;
}

export const ITOKEN_PROVIDER = Symbol('ITokenProvider');
