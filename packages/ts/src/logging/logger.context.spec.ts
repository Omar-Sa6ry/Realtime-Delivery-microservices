import { LoggerContext, LogContext } from './logger.context';

describe('LoggerContext', () => {
  it('returns no store and no trace id outside of run', () => {
    expect(LoggerContext.getStore()).toBeUndefined();
    expect(LoggerContext.getTraceId()).toBeUndefined();
  });

  it('exposes the context inside run and clears it afterwards', () => {
    LoggerContext.run({ traceId: 'trace-1', userId: 'user-1' }, () => {
      expect(LoggerContext.getStore()).toEqual({
        traceId: 'trace-1',
        userId: 'user-1',
      });
      expect(LoggerContext.getTraceId()).toBe('trace-1');
    });

    expect(LoggerContext.getStore()).toBeUndefined();
    expect(LoggerContext.getTraceId()).toBeUndefined();
  });

  it('keeps nested contexts isolated and restores the outer one', () => {
    LoggerContext.run({ traceId: 'outer' }, () => {
      LoggerContext.run({ traceId: 'inner' }, () => {
        expect(LoggerContext.getTraceId()).toBe('inner');
      });
      expect(LoggerContext.getTraceId()).toBe('outer');
    });
  });

  it('propagates the context across async boundaries', async () => {
    let seenInside: LogContext | undefined;

    await new Promise<void>((resolve) => {
      LoggerContext.run({ traceId: 'async-trace', method: 'GET' }, async () => {
        await new Promise((r) => setImmediate(r));
        seenInside = LoggerContext.getStore();
        resolve();
      });
    });

    expect(seenInside).toEqual({ traceId: 'async-trace', method: 'GET' });
    expect(LoggerContext.getStore()).toBeUndefined();
  });

  it('merges new values into the active store', () => {
    LoggerContext.run({ traceId: 'trace-a', userId: 'user-a' }, () => {
      LoggerContext.setStore({ userId: 'user-b' });
      expect(LoggerContext.getStore()).toEqual({
        traceId: 'trace-a',
        userId: 'user-b',
      });

      LoggerContext.setStore({ traceId: 'trace-b', path: '/orders' });
      expect(LoggerContext.getStore()).toEqual({
        traceId: 'trace-b',
        userId: 'user-b',
        path: '/orders',
      });
      expect(LoggerContext.getTraceId()).toBe('trace-b');
    });
  });

  it('does not leak nested store updates to the outer store', () => {
    LoggerContext.run({ traceId: 'outer' }, () => {
      LoggerContext.run({ traceId: 'inner' }, () => {
        LoggerContext.setStore({ userId: 'inner-user' });
      });

      expect(LoggerContext.getStore()).toEqual({ traceId: 'outer' });
    });
  });

  it('ignores setStore when no context is active', () => {
    expect(() => LoggerContext.setStore({ traceId: 'orphan' })).not.toThrow();
    expect(LoggerContext.getStore()).toBeUndefined();
  });
});
