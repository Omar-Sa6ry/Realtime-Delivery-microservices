import { ConflictException, Injectable } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { I18nService } from 'nestjs-i18n';

@Injectable()
export class IdempotencyService {
  constructor(
    private readonly redis: RedisService,
    private readonly i18n: I18nService,
  ) {}
  async get<T>(key: string): Promise<T | null> {
    return this.redis.get<T>(`delivery:idempotency:${key}`);
  }

  async execute<T>(
    key: string,
    operation: () => Promise<T>,
    ttlSeconds = 86400,
  ): Promise<T> {
    const storageKey = `delivery:idempotency:${key}`;
    const existing = await this.redis.get<T>(storageKey);
    if (existing !== null) return existing;
    const claimed = await this.redis.setNX(`${storageKey}:lock`, '1', 30);
    if (!claimed)
      throw new ConflictException(
        'An operation with this idempotency key is already in progress',
      );
    try {
      const result = await operation();
      await this.redis.set(storageKey, result, ttlSeconds);
      return result;
    } finally {
      await this.redis.del(`${storageKey}:lock`);
    }
  }
}
