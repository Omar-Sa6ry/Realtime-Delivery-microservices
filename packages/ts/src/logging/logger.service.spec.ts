import { StructuredLogger } from './logger.service';
import { LoggerContext } from './logger.context';

describe('StructuredLogger', () => {
  let logger: StructuredLogger;
  let logSpy: jest.SpyInstance;
  let errorSpy: jest.SpyInstance;
  let warnSpy: jest.SpyInstance;
  const originalEnv = process.env.NODE_ENV;
  const originalLevel = process.env.LOG_LEVEL;

  beforeEach(() => {
    logger = new StructuredLogger();
    process.env.NODE_ENV = 'test';
    logSpy = jest.spyOn(console, 'log').mockImplementation(() => undefined);
    errorSpy = jest.spyOn(console, 'error').mockImplementation(() => undefined);
    warnSpy = jest.spyOn(console, 'warn').mockImplementation(() => undefined);
  });

  afterEach(() => {
    jest.restoreAllMocks();
    if (originalEnv === undefined) {
      delete process.env.NODE_ENV;
    } else {
      process.env.NODE_ENV = originalEnv;
    }
    if (originalLevel === undefined) {
      delete process.env.LOG_LEVEL;
    } else {
      process.env.LOG_LEVEL = originalLevel;
    }
  });

  it('logs an INFO line with the default Application context', () => {
    logger.log('service started');

    const output = logSpy.mock.calls[0][0] as string;
    expect(output).toContain('[INFO]');
    expect(output).toContain('[Application]');
    expect(output).toContain('service started');
  });

  it('logs with an explicit context', () => {
    logger.log('authenticated', 'Auth');

    const output = logSpy.mock.calls[0][0] as string;
    expect(output).toContain('[Auth]');
    expect(output).not.toContain('[Application]');
  });

  it('includes the trace id coming from the log context', () => {
    LoggerContext.run({ traceId: 'trace-42', userId: 'user-7' }, () => {
      logger.log('with trace', 'Orders');
    });

    const output = logSpy.mock.calls[0][0] as string;
    expect(output).toContain('[TraceID: trace-42]');
    expect(output).toContain('[Orders]');
  });

  it('pretty prints object messages', () => {
    logger.log({ a: 1, b: 'two' }, 'Payload');

    const output = logSpy.mock.calls[0][0] as string;
    expect(output).toContain(JSON.stringify({ a: 1, b: 'two' }, null, 2));
  });

  it('colors ERROR, WARN and DEBUG lines', () => {
    logger.error('bad thing', undefined, 'Ctx');
    logger.warn('careful');
    logger.debug('chatter');
    logger.verbose('quiet');
    logger.log('plain');

    expect(errorSpy.mock.calls[0][0]).toContain('\x1b[31m[ERROR]');
    expect(warnSpy.mock.calls[0][0]).toContain('\x1b[33m[WARN]');
    expect(logSpy.mock.calls[0][0]).toContain('\x1b[35m[DEBUG]');
    expect(logSpy.mock.calls[1][0]).toContain('\x1b[0m[VERBOSE]');
    expect(logSpy.mock.calls[2][0]).toContain('\x1b[0m[INFO]');
  });

  it('suppresses records below LOG_LEVEL', () => {
    process.env.LOG_LEVEL = 'warn';

    logger.debug('chatter');
    logger.log('started');
    logger.warn('degraded');
    logger.error('broken');

    expect(logSpy).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalledTimes(1);
    expect(errorSpy).toHaveBeenCalledTimes(1);
  });

  it('defaults to info level in production when LOG_LEVEL is unset', () => {
    delete process.env.LOG_LEVEL;
    process.env.NODE_ENV = 'production';

    logger.debug('chatter');
    logger.log('started');

    expect(logSpy).toHaveBeenCalledTimes(1);
    expect(logSpy.mock.calls[0][0]).toContain('"level":"INFO"');
  });

  it('writes the stack trace separately when not in production', () => {
    logger.error('boom', 'Error: boom\n    at somewhere', 'Http');

    expect(errorSpy).toHaveBeenCalledTimes(2);
    expect(errorSpy.mock.calls[0][0]).toContain('[Http]');
    expect(errorSpy.mock.calls[0][0]).toContain('boom');
    expect(errorSpy.mock.calls[1][0]).toBe('Error: boom\n    at somewhere');
  });

  it('falls back to the trace argument as context', () => {
    logger.error('boom', 'fallback-trace');

    expect(errorSpy.mock.calls[0][0]).toContain('[fallback-trace]');
  });

  it('skips the extra stack line in production', () => {
    process.env.NODE_ENV = 'production';

    logger.error('boom', 'stack-line', 'Http');

    expect(errorSpy).toHaveBeenCalledTimes(1);
  });

  it('emits machine readable json in production', () => {
    process.env.NODE_ENV = 'production';

    LoggerContext.run({ traceId: 'prod-trace' }, () => {
      logger.log('shipped', 'Shipper');
    });

    const parsed = JSON.parse(logSpy.mock.calls[0][0] as string);
    expect(parsed).toMatchObject({
      level: 'INFO',
      context: 'Shipper',
      message: { text: 'shipped' },
      traceId: 'prod-trace',
    });
    expect(typeof parsed.timestamp).toBe('string');
  });

  it('keeps object payloads as objects in production json', () => {
    process.env.NODE_ENV = 'production';

    logger.log({ code: 42 });

    const parsed = JSON.parse(logSpy.mock.calls[0][0] as string);
    expect(parsed.message).toEqual({ code: 42 });
    expect(parsed.context).toBe('Application');
  });

  it('propagates log calls through the Nest LoggerService contract', () => {
    const service: any = logger as any;

    expect(typeof service.log).toBe('function');
    expect(typeof service.error).toBe('function');
    expect(typeof service.warn).toBe('function');
    expect(typeof service.debug).toBe('function');
    expect(typeof service.verbose).toBe('function');

    service.warn('heads up', 'Warn');
    expect(warnSpy.mock.calls[0][0]).toContain('[WARN]');
    expect(warnSpy.mock.calls[0][0]).toContain('[Warn]');
  });
});
