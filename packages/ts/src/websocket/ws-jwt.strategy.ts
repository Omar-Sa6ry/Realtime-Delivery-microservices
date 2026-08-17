import { Inject, Injectable, Logger } from '@nestjs/common';
import type { IncomingMessage } from 'http';
import { IJwtPayload } from '../interfaces/user.interface';
import { WsErrorCode, WsException } from './ws-errors';

export const WS_JWT_SERVICE = 'WS_JWT_SERVICE';

@Injectable()
export class WsJwtStrategy {
  private readonly logger = new Logger(WsJwtStrategy.name);

  constructor(@Inject(WS_JWT_SERVICE) private readonly jwtService: any) {}

  async authenticate(request: IncomingMessage): Promise<IJwtPayload> {
    const token = this.extractToken(request);
    if (!token) {
      throw new WsException(WsErrorCode.UNAUTHENTICATED, 'Missing token', false);
    }

    try {
      const verified = await this.jwtService.verifyAsync(token);
      return this.normalize(verified as IJwtPayload);
    } catch (err) {
      this.logger.warn(`WebSocket JWT verification failed: ${(err as Error).message}`);
      throw new WsException(WsErrorCode.UNAUTHENTICATED, 'Invalid or expired token', false);
    }
  }

  private extractToken(request: IncomingMessage): string | null {
    const authHeader = request.headers?.authorization;
    if (authHeader?.startsWith('Bearer ')) return authHeader.slice(7);

    const url = request.url || '';
    const match = url.match(/[?&]token=([^&]+)/);
    return match ? decodeURIComponent(match[1]) : null;
  }

  private normalize(payload: IJwtPayload): IJwtPayload {
    return {
      userId: payload.userId || payload.sub || payload.id,
      sub: payload.sub,
      id: payload.id,
      role: payload.role,
      email: payload.email,
      sessionId: payload.sessionId,
      iat: payload.iat,
      exp: payload.exp,
    };
  }
}