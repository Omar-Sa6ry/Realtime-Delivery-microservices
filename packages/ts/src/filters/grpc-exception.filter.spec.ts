import { RpcException } from '@nestjs/microservices';
import { status } from '@grpc/grpc-js';
import { Observable, firstValueFrom } from 'rxjs';
import { GrpcExceptionFilter } from './grpc-exception.filter';

describe('GrpcExceptionFilter', () => {
  let filter: GrpcExceptionFilter;
  let loggerSpy: jest.SpyInstance;
  const host = {} as any;

  beforeEach(() => {
    filter = new GrpcExceptionFilter();
    loggerSpy = jest.spyOn(filter['logger'], 'error').mockImplementation();
  });

  afterEach(() => {
    loggerSpy.mockRestore();
  });

  it('returns an observable', () => {
    const observable = filter.catch({ message: 'x' }, host);

    expect(observable).toBeInstanceOf(Observable);
  });

  it('propagates RpcException error untouched without logging', async () => {
    const payload = { code: status.NOT_FOUND, message: 'nope' };
    const observable = filter.catch(new RpcException(payload), host);

    await expect(firstValueFrom(observable)).rejects.toEqual(payload);
    expect(loggerSpy).not.toHaveBeenCalled();
  });

  it('propagates string error from RpcException untouched', async () => {
    const observable = filter.catch(new RpcException('raw failure'), host);

    await expect(firstValueFrom(observable)).rejects.toBe('raw failure');
    expect(loggerSpy).not.toHaveBeenCalled();
  });

  it.each([
    [400, status.INVALID_ARGUMENT],
    [401, status.UNAUTHENTICATED],
    [403, status.PERMISSION_DENIED],
    [404, status.NOT_FOUND],
    [409, status.ALREADY_EXISTS],
    [429, status.RESOURCE_EXHAUSTED],
    [500, status.INTERNAL],
    [418, status.INTERNAL],
  ])('maps http status %i to grpc code %i', async (httpStatus, grpcCode) => {
    const observable = filter.catch({ status: httpStatus, message: 'handled' }, host);

    await expect(firstValueFrom(observable)).rejects.toEqual({
      code: grpcCode,
      message: 'handled',
    });
    expect(loggerSpy).toHaveBeenCalledWith(`[gRPC Error] ${grpcCode}: handled`);
  });

  it('reads statusCode property when status is absent', async () => {
    const observable = filter.catch({ statusCode: 404, message: 'gone' }, host);

    await expect(firstValueFrom(observable)).rejects.toEqual({
      code: status.NOT_FOUND,
      message: 'gone',
    });
    expect(loggerSpy).toHaveBeenCalledWith(`[gRPC Error] ${status.NOT_FOUND}: gone`);
  });

  it('defaults to INTERNAL code when no status is provided', async () => {
    const observable = filter.catch({ message: 'oops' }, host);

    await expect(firstValueFrom(observable)).rejects.toEqual({
      code: status.INTERNAL,
      message: 'oops',
    });
  });

  it('defaults message and code when exception is empty', async () => {
    const observable = filter.catch({}, host);

    await expect(firstValueFrom(observable)).rejects.toEqual({
      code: status.INTERNAL,
      message: 'Internal server error',
    });
    expect(loggerSpy).toHaveBeenCalledWith(
      `[gRPC Error] ${status.INTERNAL}: Internal server error`,
    );
  });
});
