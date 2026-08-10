import { Injectable } from '@nestjs/common';
import { RedisService } from '@bts-soft/core';
import { ISessionRepository, UserSessionData } from '../../../domain/repositories/session.repository.interface';

@Injectable()
export class RedisSessionRepository implements ISessionRepository {
  constructor(private readonly redisService: RedisService) {}

  private sessionKey(userId: string, sessionId: string): string {
    return `user:session:${userId}:${sessionId}`;
  }

  private refreshKey(refreshTokenHash: string): string {
    return `user:refresh:${refreshTokenHash}`;
  }

  async createSession(userId: string, sessionId: string, data: UserSessionData, ttlSeconds: number): Promise<void> {
    const key = this.sessionKey(userId, sessionId);
    await this.redisService.set(key, data, ttlSeconds);
  }

  async getSession(userId: string, sessionId: string): Promise<UserSessionData | null> {
    const key = this.sessionKey(userId, sessionId);
    const data = await this.redisService.get(key);
    return data ? (data as UserSessionData) : null;
  }

  async revokeSession(userId: string, sessionId: string): Promise<void> {
    const key = this.sessionKey(userId, sessionId);
    await this.redisService.del(key);
  }

  async revokeAllUserSessions(userId: string): Promise<void> {
    const pattern = `user:session:${userId}:*`;
    const keys = await (this.redisService as any).keys?.(pattern);
    if (keys && keys.length > 0) {
      for (const k of keys) {
        await this.redisService.del(k);
      }
    }
  }

  async storeRefreshToken(refreshTokenHash: string, userId: string, ttlSeconds: number): Promise<void> {
    const key = this.refreshKey(refreshTokenHash);
    await this.redisService.set(key, { userId }, ttlSeconds);
  }

  async getUserIdByRefreshToken(refreshTokenHash: string): Promise<string | null> {
    const key = this.refreshKey(refreshTokenHash);
    const data: any = await this.redisService.get(key);
    return data?.userId || null;
  }

  async revokeRefreshToken(refreshTokenHash: string): Promise<void> {
    const key = this.refreshKey(refreshTokenHash);
    await this.redisService.del(key);
  }
}
