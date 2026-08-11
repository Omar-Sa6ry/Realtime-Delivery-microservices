import { Injectable } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { ConfigService } from '@nestjs/config';

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

@Injectable()
export class JwtTokenProvider {
  constructor(
    private readonly jwtService: JwtService,
    private readonly configService: ConfigService,
  ) {}

  async generateTokens(payload: TokenPayload): Promise<TokenResult> {
    const secret = this.configService.get<string>('JWT_SECRET') || 'default_secret';
    const expiresInSeconds = 3600 * 24; // 1 day access token

    const accessToken = await this.jwtService.signAsync(
      {
        sub: payload.userId,
        userId: payload.userId,
        email: payload.email,
        role: payload.role,
        permissions: payload.permissions,
        sessionId: payload.sessionId,
      },
      { secret, expiresIn: expiresInSeconds },
    );

    const refreshToken = await this.jwtService.signAsync(
      {
        sub: payload.userId,
        userId: payload.userId,
        sessionId: payload.sessionId,
        type: 'refresh',
      },
      { secret, expiresIn: 3600 * 24 * 7 }, // 7 days refresh token
    );

    return {
      accessToken,
      refreshToken,
      expiresIn: expiresInSeconds,
    };
  }

  async verifyAccessToken(token: string): Promise<TokenPayload | null> {
    try {
      const secret = this.configService.get<string>('JWT_SECRET') || 'default_secret';
      const decoded = await this.jwtService.verifyAsync(token, { secret });
      return {
        userId: decoded.userId || decoded.sub,
        email: decoded.email,
        role: decoded.role,
        permissions: decoded.permissions || [],
        sessionId: decoded.sessionId,
      };
    } catch {
      return null;
    }
  }

  async verifyRefreshToken(token: string): Promise<TokenPayload | null> {
    try {
      const secret = this.configService.get<string>('JWT_SECRET') || 'default_secret';
      const decoded = await this.jwtService.verifyAsync(token, { secret });
      if (decoded.type !== 'refresh') return null;
      return {
        userId: decoded.userId || decoded.sub,
        email: decoded.email || '',
        role: decoded.role || '',
        permissions: [],
        sessionId: decoded.sessionId,
      };
    } catch {
      return null;
    }
  }
}
