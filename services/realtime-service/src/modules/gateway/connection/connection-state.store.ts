import { Injectable, Logger } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { redisKeys, TTL } from '../../../common/common-types/constants';
import { ConnectionContext } from './connection.types';

/**
 * Redis-backed distributed connection metadata.
 *   ws:connection:{socketId}         -> JSON ConnectionContext (TTL: 60s, renewed by heartbeat)
 *   ws:user:{userId}:sockets         -> Set<socketId>
 *   ws:role:{role}:sockets           -> Set<socketId>  (presence fan-out to admins)
 *   ws:node:{nodeId}:connections     -> Set<socketId>
 */
@Injectable()
export class ConnectionStateStore {
  private readonly logger = new Logger(ConnectionStateStore.name);

  constructor(private readonly redis: RedisService) {}

  async register(ctx: ConnectionContext): Promise<void> {
    try {
      await this.redis.set(redisKeys.connection(ctx.socketId), ctx, TTL.CONNECTION);
      await this.redis.sAdd(redisKeys.userSockets(ctx.userId), ctx.socketId);
      await this.redis.sAdd(redisKeys.roleSockets(ctx.userRole), ctx.socketId);
      await this.redis.sAdd(
        redisKeys.nodeConnections(ctx.nodeId),
        ctx.socketId,
      );
    } catch (err) {
      this.logger.error(`Failed to register connection ${ctx.socketId}: ${err.message}`);
    }
  }

  async unregister(ctx: ConnectionContext): Promise<void> {
    try {
      await this.redis.del(redisKeys.connection(ctx.socketId));
      await this.redis.sRem(redisKeys.userSockets(ctx.userId), ctx.socketId);
      await this.redis.sRem(redisKeys.roleSockets(ctx.userRole), ctx.socketId);
      await this.redis.sRem(redisKeys.nodeConnections(ctx.nodeId), ctx.socketId);
    } catch (err) {
      this.logger.error(`Failed to unregister connection ${ctx.socketId}: ${err.message}`);
    }
  }

  async refreshHeartbeat(socketId: string): Promise<void> {
    try {
      await this.redis.expire(redisKeys.connection(socketId), TTL.CONNECTION);
    } catch (err) {
      this.logger.error(`Failed to refresh heartbeat for ${socketId}: ${err.message}`);
    }
  }

  async userConnectionCount(userId: string): Promise<number> {
    try {
      return await this.redis.sCard(redisKeys.userSockets(userId));
    } catch (err) {
      this.logger.error(`Failed to count connections for user ${userId}: ${err.message}`);
      return 0;
    }
  }

  async userConnectionCountsByRole(): Promise<Record<string, number>> {
    const roles = ['user', 'driver', 'admin'] as const;
    const result: Record<string, number> = {};
    for (const role of roles) {
      result[role] = await this.redis.sCard(redisKeys.roleSockets(role)).catch(() => 0);
    }
    return result;
  }

  async totalConnections(): Promise<number> {
    try {
      const roles = ['user', 'driver', 'admin'] as const;
      let total = 0;
      for (const role of roles) {
        total += await this.redis.sCard(redisKeys.roleSockets(role));
      }
      return total;
    } catch (err) {
      this.logger.error(`Failed to count total connections: ${err.message}`);
      return 0;
    }
  }

  async getContext(socketId: string): Promise<ConnectionContext | null> {
    try {
      return await this.redis.get<ConnectionContext>(redisKeys.connection(socketId));
    } catch (err) {
      this.logger.error(`Failed to read connection context ${socketId}: ${err.message}`);
      return null;
    }
  }
}