import { ExecutionContext, CallHandler } from '@nestjs/common';
import { Observable, firstValueFrom, of, throwError } from 'rxjs';
import { GqlExecutionContext } from '@nestjs/graphql';
import { MetricsInterceptor } from './metrics.interceptor';
import { MetricsService } from './metrics.service';

jest.mock('@nestjs/graphql', () => ({
  GqlExecutionContext: {
    create: jest.fn(),
  },
}));

const createMetricsMock = () => ({
  requestCounter: { inc: jest.fn() },
  requestDuration: { observe: jest.fn() },
  errorCounter: { inc: jest.fn() },
});

type MetricsMock = ReturnType<typeof createMetricsMock>;

function httpContext(req: any, res: any): ExecutionContext {
  return {
    getType: () => 'http',
    switchToHttp: () => ({
      getRequest: () => req,
      getResponse: () => res,
    }),
    getHandler: () => function httpHandler() {},
  } as unknown as ExecutionContext;
}

function typedContext(
  type: string,
  handlerName = 'resolverHandler',
): ExecutionContext {
  return {
    getType: () => type,
    switchToHttp: () => ({
      getRequest: () => ({}),
      getResponse: () => ({}),
    }),
    getHandler: () => {
      const fn = () => undefined;
      Object.defineProperty(fn, 'name', { value: handlerName });
      return fn;
    },
  } as unknown as ExecutionContext;
}

function handlerOf(result: unknown = 'payload'): CallHandler {
  return { handle: jest.fn(() => of(result)) } as unknown as CallHandler;
}

function failingHandler(error: unknown): CallHandler {
  return {
    handle: jest.fn(() => throwError(() => error)),
  } as unknown as CallHandler;
}

describe('MetricsInterceptor', () => {
  let metrics: MetricsMock;
  let interceptor: MetricsInterceptor;
  const gqlCreate = GqlExecutionContext.create as jest.Mock;

  beforeEach(() => {
    metrics = createMetricsMock();
    interceptor = new MetricsInterceptor(metrics as unknown as MetricsService);
    gqlCreate.mockReset();
  });

  describe('http context', () => {
    it('records labels for a successful response', async () => {
      const context = httpContext(
        { method: 'POST', baseUrl: '/api', path: '/orders' },
        { statusCode: 201 },
      );
      const handler = handlerOf('created');

      const result = await firstValueFrom(
        interceptor.intercept(context, handler),
      );

      expect(result).toBe('created');
      expect(metrics.requestCounter.inc).toHaveBeenCalledWith({
        protocol: 'HTTP',
        method: 'POST',
        path: '/api/orders',
        statusCode: '201',
      });
      expect(metrics.requestDuration.observe).toHaveBeenCalledWith(
        {
          protocol: 'HTTP',
          method: 'POST',
          path: '/api/orders',
          statusCode: '201',
        },
        expect.any(Number),
      );
      expect(metrics.errorCounter.inc).not.toHaveBeenCalled();
    });

    it('defaults the status code when the response has none', async () => {
      const context = httpContext(
        { method: 'GET', baseUrl: '', path: '/' },
        {},
      );

      await firstValueFrom(interceptor.intercept(context, handlerOf()));

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith(
        expect.objectContaining({ statusCode: '200', path: '/' }),
      );
    });

    it('records the error status code and rethrows', async () => {
      const context = httpContext(
        { method: 'GET', baseUrl: '/api', path: '/orders' },
        { statusCode: 200 },
      );
      const error = Object.assign(new Error('missing'), {
        status: 404,
        name: 'NotFoundError',
      });

      await expect(
        firstValueFrom(interceptor.intercept(context, failingHandler(error))),
      ).rejects.toBe(error);

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith({
        protocol: 'HTTP',
        method: 'GET',
        path: '/api/orders',
        statusCode: 404,
      });
      expect(metrics.requestDuration.observe).toHaveBeenCalledWith(
        expect.objectContaining({ statusCode: 404 }),
        expect.any(Number),
      );
      expect(metrics.errorCounter.inc).toHaveBeenCalledWith({
        context: 'HTTP:/api/orders',
        errorCode: 'NotFoundError',
      });
    });

    it('falls back to statusCode then 500 on failures', async () => {
      const context = httpContext(
        { method: 'DELETE', baseUrl: '/api', path: '/orders/1' },
        { statusCode: 200 },
      );
      const error = { statusCode: 503, code: 'UNAVAILABLE' };

      await expect(
        firstValueFrom(interceptor.intercept(context, failingHandler(error))),
      ).rejects.toBe(error);

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith(
        expect.objectContaining({ statusCode: 503 }),
      );
      expect(metrics.errorCounter.inc).toHaveBeenCalledWith({
        context: 'HTTP:/api/orders/1',
        errorCode: 'UNAVAILABLE',
      });
    });

    it('falls back to 500 and UNKNOWN_ERROR for bare failures', async () => {
      const context = httpContext(
        { method: 'GET', baseUrl: '', path: '/boom' },
        { statusCode: 200 },
      );
      const error = {};

      await expect(
        firstValueFrom(interceptor.intercept(context, failingHandler(error))),
      ).rejects.toBe(error);

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith(
        expect.objectContaining({ statusCode: '500' }),
      );
      expect(metrics.errorCounter.inc).toHaveBeenCalledWith({
        context: 'HTTP:/boom',
        errorCode: 'UNKNOWN_ERROR',
      });
    });
  });

  describe('graphql context', () => {
    it('records query operations with the field name as path', async () => {
      gqlCreate.mockReturnValue({
        getInfo: () => ({
          operation: { operation: 'query' },
          fieldName: 'searchOrders',
        }),
      });

      const result = await firstValueFrom(
        interceptor.intercept(typedContext('graphql'), handlerOf('rows')),
      );

      expect(gqlCreate).toHaveBeenCalled();
      expect(result).toBe('rows');
      expect(metrics.requestCounter.inc).toHaveBeenCalledWith({
        protocol: 'GraphQL',
        method: 'QUERY',
        path: 'searchOrders',
        statusCode: '200',
      });
    });

    it('uppercases mutation operations', async () => {
      gqlCreate.mockReturnValue({
        getInfo: () => ({
          operation: { operation: 'mutation' },
          fieldName: 'createOrder',
        }),
      });

      await firstValueFrom(
        interceptor.intercept(typedContext('graphql'), handlerOf()),
      );

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith(
        expect.objectContaining({ method: 'MUTATION', path: 'createOrder' }),
      );
    });

    it('defaults to QUERY when the operation is unknown', async () => {
      gqlCreate.mockReturnValue({
        getInfo: () => ({ operation: undefined, fieldName: 'orders' }),
      });

      await firstValueFrom(
        interceptor.intercept(typedContext('graphql'), handlerOf()),
      );

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith(
        expect.objectContaining({ method: 'QUERY', path: 'orders' }),
      );
    });

    it('records graphql failures and rethrows', async () => {
      gqlCreate.mockReturnValue({
        getInfo: () => ({
          operation: { operation: 'query' },
          fieldName: 'order',
        }),
      });
      const error = Object.assign(new Error('resolver failed'), {
        code: 'RESOLVER_ERROR',
      });

      await expect(
        firstValueFrom(
          interceptor.intercept(typedContext('graphql'), failingHandler(error)),
        ),
      ).rejects.toBe(error);

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith(
        expect.objectContaining({
          protocol: 'GraphQL',
          method: 'QUERY',
          path: 'order',
          statusCode: '500',
        }),
      );
      expect(metrics.errorCounter.inc).toHaveBeenCalledWith({
        context: 'GraphQL:order',
        errorCode: 'RESOLVER_ERROR',
      });
    });

    it('falls back when the graphql execution context cannot be built', async () => {
      gqlCreate.mockImplementation(() => {
        throw new Error('no schema');
      });

      await firstValueFrom(
        interceptor.intercept(typedContext('graphql'), handlerOf()),
      );

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith({
        protocol: 'GraphQL',
        method: 'UNKNOWN',
        path: 'GraphQLResolver',
        statusCode: '200',
      });
    });

    it('records failures for the graphql fallback path', async () => {
      gqlCreate.mockImplementation(() => {
        throw new Error('no schema');
      });
      const error = Object.assign(new Error('boom'), { name: 'BoomError' });

      await expect(
        firstValueFrom(
          interceptor.intercept(typedContext('graphql'), failingHandler(error)),
        ),
      ).rejects.toBe(error);

      expect(metrics.errorCounter.inc).toHaveBeenCalledWith({
        context: 'GraphQL:GraphQLResolver',
        errorCode: 'BoomError',
      });
    });
  });

  describe('rpc context', () => {
    it('records the handler name as path on success', async () => {
      const result = await firstValueFrom(
        interceptor.intercept(typedContext('rpc', 'CreateOrder'), handlerOf('ok')),
      );

      expect(result).toBe('ok');
      expect(metrics.requestCounter.inc).toHaveBeenCalledWith({
        protocol: 'gRPC',
        method: 'RPC',
        path: 'CreateOrder',
        statusCode: '200',
      });
    });

    it('records failures with a 500 status code', async () => {
      const error = Object.assign(new Error('grpc failed'), { code: '14' });

      await expect(
        firstValueFrom(
          interceptor.intercept(
            typedContext('rpc', 'CreateOrder'),
            failingHandler(error),
          ),
        ),
      ).rejects.toBe(error);

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith(
        expect.objectContaining({ protocol: 'gRPC', statusCode: '500' }),
      );
      expect(metrics.errorCounter.inc).toHaveBeenCalledWith({
        context: 'gRPC:CreateOrder',
        errorCode: '14',
      });
    });
  });

  describe('other contexts', () => {
    it('uses default labels for unknown context types', async () => {
      await firstValueFrom(
        interceptor.intercept(typedContext('ws'), handlerOf()),
      );

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith({
        protocol: 'HTTP',
        method: 'GET',
        path: '/',
        statusCode: '200',
      });
    });

    it('records failures for unknown context types', async () => {
      const error = new Error('ws failure');

      await expect(
        firstValueFrom(
          interceptor.intercept(typedContext('ws'), failingHandler(error)),
        ),
      ).rejects.toBe(error);

      expect(metrics.requestCounter.inc).toHaveBeenCalledWith(
        expect.objectContaining({ statusCode: '500' }),
      );
      expect(metrics.errorCounter.inc).toHaveBeenCalledWith({
        context: 'HTTP:/',
        errorCode: 'Error',
      });
    });

    it('returns an observable that can be subscribed multiple times', (done) => {
      const context = typedContext('ws');
      const observable: Observable<string> = interceptor.intercept(
        context,
        handlerOf('value'),
      );

      let first: string | undefined;
      observable.subscribe((v) => {
        first = v;
        observable.subscribe((second) => {
          expect(first).toBe('value');
          expect(second).toBe('value');
          expect(metrics.requestCounter.inc).toHaveBeenCalledTimes(2);
          done();
        });
      });
    });
  });
});
