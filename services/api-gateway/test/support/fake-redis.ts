/**
 * In-memory stand-ins for the Redis-backed providers the gateway wires in
 * (`RedisService`, `REDIS_CLIENT`, `CACHE_MANAGER`).
 *
 * The real `@bts-soft/cache` RedisModule:
 *  - connects a `redis` client in a factory (would block boot without Redis),
 *  - runs `RedisHealth.onModuleInit()` which calls `process.exit(1)` if
 *    `redisClient.ping()` rejects — so the fake client MUST resolve `ping()`,
 *  - is consumed by the global `RateLimiter` guard through `RedisStore`, which
 *    only calls `get` / `set` / `del` on the injected `RedisService`.
 *
 * These fakes let the real gateway boot and the real token-bucket rate limiter
 * run, without any external process.
 */

interface Entry {
  value: string;
  expiresAt?: number;
}

export class FakeRedisService {
  private store = new Map<string, Entry>();

  async get(key: string): Promise<any> {
    const entry = this.store.get(key);
    if (!entry) return null;
    if (entry.expiresAt && Date.now() > entry.expiresAt) {
      this.store.delete(key);
      return null;
    }
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
  }

  async exists(key: string): Promise<boolean> {
    return this.store.has(key);
  }

  async incr(key: string): Promise<number> {
    const current = (await this.get(key)) ?? 0;
    const next = Number(current) + 1;
    await this.set(key, next);
    return next;
  }

  async ping(): Promise<string> {
    return 'PONG';
  }

  /** Test helper: wipe everything. */
  flush(): void {
    this.store.clear();
  }

  /** Test helper: number of live keys. */
  size(): number {
    return this.store.size;
  }
}

/**
 * A `redis`-client shaped object. Only the methods touched during boot /
 * health checks are implemented; anything else is a no-op that resolves so the
 * gateway never crashes.
 */
export function createFakeRedisClient() {
  return {
    isOpen: true,
    ping: async () => 'PONG',
    get: async () => null,
    set: async () => 'OK',
    del: async () => 0,
    quit: async () => 'OK',
    disconnect: () => undefined,
    on: () => undefined,
    multi: () => ({
      set: () => ({ exec: async () => [] }),
      exec: async () => [],
    }),
    sendCommand: async () => null,
  };
}

/** A `cache-manager` shaped object (only used if something bypasses override). */
export function createFakeCacheManager() {
  const store = new Map<string, unknown>();
  return {
    get: async (key: string) => store.get(key),
    set: async (key: string, value: unknown) => {
      store.set(key, value);
    },
    del: async (key: string) => {
      store.delete(key);
    },
    clear: async () => store.clear(),
  };
}
