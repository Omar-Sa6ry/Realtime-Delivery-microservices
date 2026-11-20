import { EventEmitter } from 'events';
import { waitForService, waitForRedis } from './waitService.util';

jest.mock('@delivery-micro/shard', () => ({
  StructuredLogger: jest.fn().mockImplementation(() => ({
    log: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
  })),
}));

// Replace the whole `net` module — Node's real exports are non-configurable so
// jest.spyOn cannot wrap them. waitService only uses net.createConnection.
jest.mock('net', () => ({ createConnection: jest.fn() }));

const net = require('net');

/** A fake socket we can drive through connect / error / timeout. */
class FakeSocket extends EventEmitter {
  destroyed = false;
  timeoutMs?: number;
  timeoutCb?: () => void;

  destroy() {
    this.destroyed = true;
  }

  setTimeout(ms: number, cb: () => void) {
    this.timeoutMs = ms;
    this.timeoutCb = cb;
    return this;
  }
}

describe('waitForService', () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
    jest.restoreAllMocks();
  });

  it('resolves immediately when the service responds with __typename', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: { __typename: 'Query' } }),
    }) as any;

    await expect(
      waitForService('http://svc', undefined, 1, 2),
    ).resolves.toBeUndefined();
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it('POSTs an introspection probe query', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: { __typename: 'Query' } }),
    }) as any;

    await waitForService('http://svc', undefined, 1, 2);

    const [url, init] = (global.fetch as jest.Mock).mock.calls[0];
    expect(url).toBe('http://svc');
    expect(init.method).toBe('POST');
    expect(init.body).toContain('__typename');
  });

  it('retries until the service becomes ready', async () => {
    global.fetch = jest
      .fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ data: {} }) })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { __typename: 'Query' } }),
      }) as any;

    await waitForService('http://svc', undefined, 1, 2);

    expect(global.fetch).toHaveBeenCalledTimes(2);
  });

  it('retries after a non-ok response', async () => {
    global.fetch = jest
      .fn()
      .mockResolvedValueOnce({ ok: false, json: async () => ({}) })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { __typename: 'Query' } }),
      }) as any;

    await waitForService('http://svc', undefined, 1, 2);

    expect(global.fetch).toHaveBeenCalledTimes(2);
  });

  it('retries after a network error then succeeds', async () => {
    global.fetch = jest
      .fn()
      .mockRejectedValueOnce(new Error('ECONNREFUSED'))
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { __typename: 'Query' } }),
      }) as any;

    await expect(
      waitForService('http://svc', undefined, 1, 2),
    ).resolves.toBeUndefined();
    expect(global.fetch).toHaveBeenCalledTimes(2);
  });

  it('gives up (returns) when the signal is aborted', async () => {
    const controller = new AbortController();
    global.fetch = jest.fn().mockImplementation(async () => {
      controller.abort();
      throw new Error('aborted');
    }) as any;

    await expect(
      waitForService('http://svc', controller.signal, 1, 2),
    ).resolves.toBeUndefined();
  });

  it('uses default delay/maxDelay when only a url is given', async () => {
    // Succeeds on the first attempt so the 5s default delay never sleeps, but
    // the default-parameter branches are still exercised.
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: { __typename: 'Query' } }),
    }) as any;

    await expect(waitForService('http://svc')).resolves.toBeUndefined();
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it('backs off but caps the delay at maxDelay', async () => {
    const fetchMock = jest
      .fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ data: {} }) })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ data: {} }) })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { __typename: 'Query' } }),
      });
    global.fetch = fetchMock as any;

    const start = Date.now();
    await waitForService('http://svc', undefined, 1, 1);
    const elapsed = Date.now() - start;

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(elapsed).toBeLessThan(2000);
  });
});

describe('waitForRedis', () => {
  beforeEach(() => {
    net.createConnection.mockReset();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it('resolves when the socket connects on the first attempt', async () => {
    const sockets: FakeSocket[] = [];
    net.createConnection.mockImplementation(() => {
      const socket = new FakeSocket();
      sockets.push(socket);
      setImmediate(() => socket.emit('connect'));
      return socket;
    });

    await expect(waitForRedis('redis-host', 6379)).resolves.toBeUndefined();

    expect(net.createConnection).toHaveBeenCalledWith({
      host: 'redis-host',
      port: 6379,
    });
    expect(sockets[0].destroyed).toBe(true);
  });

  it('retries after a connection error then succeeds (~2s backoff)', async () => {
    jest.useFakeTimers();
    let attempts = 0;
    net.createConnection.mockImplementation(() => {
      attempts += 1;
      const socket = new FakeSocket();
      if (attempts === 1) {
        setImmediate(() => socket.emit('error', new Error('down')));
      } else {
        setImmediate(() => socket.emit('connect'));
      }
      return socket;
    });

    const promise = waitForRedis('h', 1);

    // First attempt errors → loop sleeps 2s (fake) then retries.
    await jest.advanceTimersByTimeAsync(2100);

    await expect(promise).resolves.toBeUndefined();
    expect(net.createConnection).toHaveBeenCalledTimes(2);
  });

  it('retries after a connection timeout then succeeds', async () => {
    jest.useFakeTimers();
    let attempts = 0;
    net.createConnection.mockImplementation(() => {
      attempts += 1;
      const socket = new FakeSocket();
      if (attempts === 1) {
        // Drive the setTimeout(2000) callback that canConnect registers.
        setImmediate(() => socket.timeoutCb?.());
      } else {
        setImmediate(() => socket.emit('connect'));
      }
      return socket;
    });

    const promise = waitForRedis('h', 1);

    await jest.advanceTimersByTimeAsync(2100);

    await expect(promise).resolves.toBeUndefined();
    expect(net.createConnection).toHaveBeenCalledTimes(2);
  });

  it('uses REDIS_HOST / REDIS_PORT env defaults when args are omitted', async () => {
    process.env.REDIS_HOST = 'env-host';
    process.env.REDIS_PORT = '7001';
    net.createConnection.mockImplementation(() => {
      const socket = new FakeSocket();
      setImmediate(() => socket.emit('connect'));
      return socket;
    });

    await waitForRedis();

    expect(net.createConnection).toHaveBeenCalledWith({
      host: 'env-host',
      port: 7001,
    });

    delete process.env.REDIS_HOST;
    delete process.env.REDIS_PORT;
  });

  it('falls back to redis-srv:6379 when env vars are unset', async () => {
    delete process.env.REDIS_HOST;
    delete process.env.REDIS_PORT;
    net.createConnection.mockImplementation(() => {
      const socket = new FakeSocket();
      setImmediate(() => socket.emit('connect'));
      return socket;
    });

    await waitForRedis();

    expect(net.createConnection).toHaveBeenCalledWith({
      host: 'redis-srv',
      port: 6379,
    });
  });
});
