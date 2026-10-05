import { WS_DEFAULTS } from './ws-constants';

describe('WS_DEFAULTS', () => {
  it('exposes the expected defaults', () => {
    expect(WS_DEFAULTS).toEqual({
      MAX_PAYLOAD: 16384,
      HEARTBEAT_INTERVAL_MS: 30000,
      HEARTBEAT_TIMEOUT_MS: 60000,
      CONNECTION_TTL_SECONDS: 60,
      PRESENCE_TTL_SECONDS: 90,
      MAX_BACKLOG: 200,
      SLOW_CONSUMER_THRESHOLD_MS: 1000,
      DEFAULT_PAGE_SIZE: 50,
    });
  });

  it('contains exactly the eight known keys', () => {
    expect(Object.keys(WS_DEFAULTS).sort()).toEqual(
      [
        'CONNECTION_TTL_SECONDS',
        'DEFAULT_PAGE_SIZE',
        'HEARTBEAT_INTERVAL_MS',
        'HEARTBEAT_TIMEOUT_MS',
        'MAX_BACKLOG',
        'MAX_PAYLOAD',
        'PRESENCE_TTL_SECONDS',
        'SLOW_CONSUMER_THRESHOLD_MS',
      ].sort(),
    );
  });

  it('keeps numeric values for every default', () => {
    for (const value of Object.values(WS_DEFAULTS)) {
      expect(typeof value).toBe('number');
      expect(Number.isFinite(value)).toBe(true);
      expect(value).toBeGreaterThan(0);
    }
  });

  it('is a plain mutable object at runtime', () => {
    expect(Object.isFrozen(WS_DEFAULTS)).toBe(false);
    expect(WS_DEFAULTS.MAX_PAYLOAD).toBe(16384);
  });
});
