import { CallHandler, ExecutionContext } from '@nestjs/common';
import { Observable, firstValueFrom, of, throwError } from 'rxjs';
import { GraphQLResponseInterceptor } from './graphql-response.interceptor';

describe('GraphQLResponseInterceptor', () => {
  const interceptor = new GraphQLResponseInterceptor();

  const contextOfType = (type: string): ExecutionContext =>
    ({
      getType: () => type,
      getHandler: () => () => undefined,
      getClass: () => class {},
    }) as unknown as ExecutionContext;

  const handlerOf = (source: Observable<any>): CallHandler => ({
    handle: jest.fn(() => source),
  });

  it('returns an observable', () => {
    const stream = interceptor.intercept(contextOfType('graphql'), handlerOf(of({ a: 1 })));

    expect(stream).toBeInstanceOf(Observable);
  });

  it('passes data through unchanged for non graphql context', async () => {
    const payload = { id: '1' };
    const next = handlerOf(of(payload));

    const result = await firstValueFrom(interceptor.intercept(contextOfType('http'), next));

    expect(result).toBe(payload);
    expect(next.handle).toHaveBeenCalledTimes(1);
  });

  it('wraps plain data into success envelope for graphql context', async () => {
    const payload = { id: '1' };
    const next = handlerOf(of(payload));

    const result = await firstValueFrom(interceptor.intercept(contextOfType('graphql'), next));

    expect(result.success).toBe(true);
    expect(result.statusCode).toBe(200);
    expect(result.message).toBe('Request successful');
    expect(result.data).toEqual(payload);
    expect(new Date(result.timeStamp).toISOString()).toBe(result.timeStamp);
  });

  it('uses payload message inside the envelope', async () => {
    const next = handlerOf(of({ message: 'Delivered', id: '2' }));

    const result = await firstValueFrom(interceptor.intercept(contextOfType('graphql'), next));

    expect(result.message).toBe('Delivered');
    expect(result.data).toEqual({ message: 'Delivered', id: '2' });
  });

  it('returns already formatted envelope untouched', async () => {
    const envelope = {
      success: true,
      statusCode: 201,
      message: 'Created',
      data: { id: '3' },
    };
    const next = handlerOf(of(envelope));

    const result = await firstValueFrom(interceptor.intercept(contextOfType('graphql'), next));

    expect(result).toBe(envelope);
  });

  it('wraps array payloads keeping items in both data and items', async () => {
    const next = handlerOf(of([1, 2, 3]));

    const result = await firstValueFrom(interceptor.intercept(contextOfType('graphql'), next));

    expect(result.data).toEqual([1, 2, 3]);
    expect(result.items).toEqual([1, 2, 3]);
    expect(result.success).toBe(true);
  });

  it('wraps null payload', async () => {
    const next = handlerOf(of(null));

    const result = await firstValueFrom(interceptor.intercept(contextOfType('graphql'), next));

    expect(result.success).toBe(true);
    expect(result.data).toBeNull();
  });

  it('propagates errors from the handler untouched', async () => {
    const error = new Error('resolver failed');
    const next = handlerOf(throwError(() => error));

    await expect(
      firstValueFrom(interceptor.intercept(contextOfType('graphql'), next)),
    ).rejects.toBe(error);
  });

  it('propagates non error rejections from the handler', async () => {
    const payload = { code: 'FORBIDDEN' };
    const next = handlerOf(throwError(() => payload));

    await expect(
      firstValueFrom(interceptor.intercept(contextOfType('graphql'), next)),
    ).rejects.toEqual(payload);
  });
});
