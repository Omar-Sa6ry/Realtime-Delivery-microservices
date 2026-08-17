import { Injectable, Logger } from '@nestjs/common';
import type { IncomingMessage } from 'http';
import type { WebSocket } from 'ws';
import { IJwtPayload } from '../interfaces/user.interface';
import { WsJwtStrategy } from './ws-jwt.strategy';

@Injectable()
export class WsAuthGuard {
  private readonly logger = new Logger(WsAuthGuard.name);

  constructor(private readonly strategy: WsJwtStrategy) {}

  async authenticate(
    socket: WebSocket,
    request: IncomingMessage,
  ): Promise<IJwtPayload | null> {
    try {
      const payload = await this.strategy.authenticate(request);
      return payload;
    } catch (err) {
      this.logger.warn(`WebSocket handshake rejected: ${(err as Error).message}`);
      socket.close(4401, 'UNAUTHENTICATED');
      return null;
    }
  }
}