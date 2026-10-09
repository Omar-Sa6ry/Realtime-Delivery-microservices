import { Request, Response, NextFunction } from 'express';
import { LoggerMiddleware } from './logger.middleware';
import { LoggerContext } from './logger.context';
import { StructuredLogger } from './logger.service';

function buildRequest(overrides: Partial<Record<string, any>> = {}): Request {
  return {
    headers: {},
    method: 'GET',
    baseUrl: '/api',
    path: '/orders',
    ...overrides,
  } as unknown as Request;
}

function buildResponse(): Response & { setHeader: jest.Mock } {
  return { setHeader: jest.fn() } as any;
}

describe('LoggerMiddleware', () => {
  let middleware: LoggerMiddleware;

  beforeEach(() => {
    middleware = new LoggerMiddleware();
  });

  it('propagates the x-trace-id header into the response and the context', () => {
    const req = buildRequest({ headers: { 'x-trace-id': 'trace-abc' } });
    const res = buildResponse();
    const next = jest.fn();

    middleware.use(req, res, next);

    expect(res.setHeader).toHaveBeenCalledWith('x-trace-id', 'trace-abc');
    expect(next).toHaveBeenCalledTimes(1);
    expect(LoggerContext.getStore()).toBeUndefined();
  });

  it('runs next() with request metadata available in the store', () => {
    const req = buildRequest({
      headers: { 'x-trace-id': 'trace-abc', 'x-user-id': 'user-9' },
      method: 'POST',
    });
    const res = buildResponse();
    let store: any;

    middleware.use(req, res, () => {
      store = LoggerContext.getStore();
    });

    expect(store).toEqual({
      traceId: 'trace-abc',
      userId: 'user-9',
      method: 'POST',
      path: '/api/orders',
    });
  });

  it('prefers the platform x-correlation-id header', () => {
    const req = buildRequest({
      headers: {
        'x-correlation-id': 'corr-1',
        'x-trace-id': 'trace-legacy',
        'x-request-id': 'req-legacy',
      },
    });
    const res = buildResponse();
    let traceId: string | undefined;

    middleware.use(req, res, () => {
      traceId = LoggerContext.getTraceId();
    });

    expect(traceId).toBe('corr-1');
    expect(res.setHeader).toHaveBeenCalledWith('x-correlation-id', 'corr-1');
    expect(res.setHeader).toHaveBeenCalledWith('x-trace-id', 'corr-1');
  });

  it('falls back to the x-request-id header', () => {
    const req = buildRequest({ headers: { 'x-request-id': 'req-123' } });
    const res = buildResponse();
    let traceId: string | undefined;

    middleware.use(req, res, () => {
      traceId = LoggerContext.getTraceId();
    });

    expect(traceId).toBe('req-123');
    expect(res.setHeader).toHaveBeenCalledWith('x-trace-id', 'req-123');
  });

  it('generates a uuid when no trace headers are present', () => {
    const req = buildRequest();
    const res = buildResponse();
    let traceId: string | undefined;

    middleware.use(req, res, () => {
      traceId = LoggerContext.getTraceId();
    });

    expect(traceId).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
    );
    expect(res.setHeader).toHaveBeenCalledWith('x-trace-id', traceId);
  });

  it('lets StructuredLogger pick up the trace id while handling the request', () => {
    const req = buildRequest({ headers: { 'x-trace-id': 'trace-log' } });
    const res = buildResponse();
    const logger = new StructuredLogger();
    const consoleSpy = jest
      .spyOn(console, 'log')
      .mockImplementation(() => undefined);

    middleware.use(req, res, () => {
      logger.log('request handled');
    });

    expect(consoleSpy).toHaveBeenCalledTimes(1);
    expect(consoleSpy.mock.calls[0][0]).toContain('[TraceID: trace-log]');
    expect(consoleSpy.mock.calls[0][0]).toContain('request handled');
    expect(LoggerContext.getStore()).toBeUndefined();

    consoleSpy.mockRestore();
  });

  it('restores the context and rethrows when next() blows up', () => {
    const req = buildRequest({ headers: { 'x-trace-id': 'trace-fail' } });
    const res = buildResponse();
    const next = jest.fn(() => {
      throw new Error('handler exploded');
    });

    expect(() => middleware.use(req, res, next as unknown as NextFunction)).toThrow(
      'handler exploded',
    );
    expect(next).toHaveBeenCalledTimes(1);
    expect(LoggerContext.getStore()).toBeUndefined();
    expect(LoggerContext.getTraceId()).toBeUndefined();
  });
});
