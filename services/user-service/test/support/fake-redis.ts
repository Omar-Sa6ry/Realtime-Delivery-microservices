/**
 * In-memory stand-in for the `RedisService` exposed by `@bts-soft/cache`.
 *
 * Why a fake is required:
 *  - `RedisSessionRepository` persists refresh/access sessions (`get/set/del`).
 *  - `UserService` caches user documents in a hash (`hSet/hGet/expire/del`).
 *  - `HealthController` pings Redis through `HealthService.checkRedis`.
 *  - `RedisRateLimiter` (register/login/forget/reset) builds a `RedisStore`
 *    from `redisService.redisClient`, so that client needs `get/set/del`.
 *
 * The e2e suite keeps the real production code paths for all of the above but
 * swaps the transport for a map, which makes the suite hermetic (only Postgres
 * is required) and lets each test start from a clean slate via `flush()`.
 *
 * Semantics deliberately mirror the real `RedisService`:
 *  - `set()` JSON-serialises, `get()` JSON-parses and returns `null` on miss.
 *  - values expire after their TTL (seconds), like ioredis would.
 *  - `hSet/hGet` operate on a JSON object stored under the key.
 */

interface Entry {
  value: string;
  expiresAt?: number;
}

export class FakeRedisService {
  private readonly store = new Map<string, Entry>();
  private readonly hashes = new Map<string, Record<string, string>>();

  /** Used by `HealthService.checkRedis` (`redisService.ping()`). */
  async ping(): Promise<string> {
    return 'PONG';
  }

  /** Shape expected by `HealthService.checkRedis` when `ping` is absent. */
  readonly client = {
    ping: async (): Promise<string> => 'PONG',
  };

  /** Shape handed to `RedisStore` by the `RedisRateLimiter` guard. */
  readonly redisClient = {
    get: async (key: string): Promise<unknown> => this.get(key),
    set: async (
      key: string,
      value: unknown,
      ttlSeconds?: number,
    ): Promise<'OK'> => {
      await this.set(key, value, ttlSeconds);
      return 'OK';
    },
    del: async (key: string): Promise<number> => {
      await this.del(key);
      return 1;
    },
  };

  private live(entry: Entry | undefined): Entry | undefined {
    if (!entry) return undefined;
    if (entry.expiresAt && Date.now() > entry.expiresAt) return undefined;
    return entry;
  }

  async get(key: string): Promise<any> {
    const entry = this.live(this.store.get(key));
    if (!entry) return null;
    try {
      return JSON.parse(entry.value);
    } catch {
      return entry.value;
    }
  }

  async set(key: string, value: unknown, ttlSeconds = 3600): Promise<void> {
    const serialized =
      typeof value === 'string' ? value : JSON.stringify(value);
    const entry: Entry = { value: serialized };
    if (ttlSeconds && ttlSeconds > 0) {
      entry.expiresAt = Date.now() + ttlSeconds * 1000;
    }
    this.store.set(key, entry);
  }

  async del(key: string): Promise<void> {
    this.store.delete(key);
    this.hashes.delete(key);
  }

  async exists(key: string): Promise<boolean> {
    return this.store.has(key) || this.hashes.has(key);
  }

  async expire(key: string, ttlSeconds: number): Promise<void> {
    const entry = this.store.get(key);
    if (entry) {
      entry.expiresAt = Date.now() + ttlSeconds * 1000;
    }
  }

  async hSet(key: string, field: string, value: unknown): Promise<void> {
    const hash = this.hashes.get(key) ?? {};
    hash[field] = typeof value === 'string' ? value : JSON.stringify(value);
    this.hashes.set(key, hash);
  }

  async hGet<T = unknown>(key: string, field: string): Promise<T | null> {
    const hash = this.hashes.get(key);
    if (!hash || !(field in hash)) return null;
    return hash[field] as T;
  }

  /** Supports the `prefix:*` glob used by `revokeAllUserSessions`. */
  async keys(pattern: string): Promise<string[]> {
    const source = pattern.replace(/[.+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp(`^${source.replace(/\*/g, '.*')}$`);
    const all = [...this.store.keys(), ...this.hashes.keys()];
    return [...new Set(all)].filter((key) => regex.test(key));
  }

  /** Test helper: drop every key (sessions, caches, rate-limit counters). */
  flush(): void {
    this.store.clear();
    this.hashes.clear();
  }

  /** Test helper: how many keys are currently alive. */
  size(): number {
    return this.store.size + this.hashes.size;
  }
}

export function createFakeRedis(): FakeRedisService {
  return new FakeRedisService();
}
