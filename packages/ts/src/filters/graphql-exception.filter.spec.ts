import { BadRequestException, HttpException, UnauthorizedException } from '@nestjs/common';
import { GraphQLError } from 'graphql';
import { GraphQLExceptionFilter } from './graphql-exception.filter';

describe('GraphQLExceptionFilter', () => {
  let filter: GraphQLExceptionFilter;
  let loggerSpy: jest.SpyInstance;
  const host = {} as any;

  beforeEach(() => {
    filter = new GraphQLExceptionFilter();
    loggerSpy = jest.spyOn(filter['logger'], 'error').mockImplementation();
  });

  afterEach(() => {
    loggerSpy.mockRestore();
  });

  it('converts UnauthorizedException into standard error envelope', () => {
    const exception = new UnauthorizedException('missing token');
    const result = filter.catch(exception, host);

    expect(result).toBeInstanceOf(GraphQLError);
    expect(result.message).toBe('missing token');
    expect(result.extensions).toMatchObject({
      success: false,
      statusCode: 401,
      code: 'INTERNAL_SERVER_ERROR',
      error: 'Unauthorized',
    });
    expect(typeof result.extensions.timeStamp).toBe('string');
    expect(new Date(result.extensions.timeStamp as string).toISOString()).toBe(
      result.extensions.timeStamp,
    );
    expect(loggerSpy).toHaveBeenCalledWith(
      'GraphQL Exception: missing token',
      exception.stack,
    );
  });

  it('maps 400 status to BAD_REQUEST code', () => {
    const result = filter.catch(new BadRequestException('bad input'), host);

    expect(result.extensions).toMatchObject({
      statusCode: 400,
      code: 'BAD_REQUEST',
      error: 'Bad Request',
    });
    expect(result.message).toBe('bad input');
  });

  it('uses extensions.statusCode from a plain error', () => {
    const exception = Object.assign(new Error('not found'), {
      extensions: { statusCode: 404 },
    });

    const result = filter.catch(exception, host);

    expect(result.extensions).toMatchObject({
      statusCode: 404,
      code: 'INTERNAL_SERVER_ERROR',
    });
    expect(result.message).toBe('not found');
    expect(loggerSpy).toHaveBeenCalledWith('GraphQL Exception: not found', exception.stack);
  });

  it('preserves provided extensions code', () => {
    const exception = Object.assign(new Error('forbidden'), {
      extensions: { code: 'FORBIDDEN', statusCode: 403 },
    });

    const result = filter.catch(exception, host);

    expect(result.extensions).toMatchObject({
      code: 'FORBIDDEN',
      statusCode: 403,
      error: 'Unknown error',
    });
  });

  it('defaults plain error to 500 and logs stack', () => {
    const exception = new Error('kaboom');

    const result = filter.catch(exception, host);

    expect(result.message).toBe('kaboom');
    expect(result.extensions).toMatchObject({
      statusCode: 500,
      code: 'INTERNAL_SERVER_ERROR',
      error: 'Unknown error',
    });
    expect(loggerSpy).toHaveBeenCalledWith('GraphQL Exception: kaboom', exception.stack);
  });

  it('handles non-error object with status field', () => {
    const exception = { status: 403, message: 'denied' };

    const result = filter.catch(exception, host);

    expect(result.message).toBe('denied');
    expect(result.extensions).toMatchObject({ statusCode: 403, success: false });
    expect(loggerSpy).toHaveBeenCalledWith('GraphQL Exception', JSON.stringify(exception));
  });

  it('handles non-error object exposing getResponse()', () => {
    const exception = {
      status: 422,
      message: 'raw message',
      getResponse: () => ({ message: 'interpreted message' }),
    };

    const result = filter.catch(exception, host);

    expect(result.message).toBe('interpreted message');
    expect(result.extensions).toMatchObject({ statusCode: 422 });
  });

  it('handles object with response payload and statusCode', () => {
    const exception = {
      statusCode: 401,
      message: 'raw',
      response: { message: 'from response', error: 'Unauthorized' },
    };

    const result = filter.catch(exception, host);

    expect(result.message).toBe('from response');
    expect(result.extensions).toMatchObject({
      statusCode: 401,
      error: 'Unauthorized',
    });
  });

  it('falls back to exception message when response has no message', () => {
    const exception = { status: 400, message: 'fallback message', response: { foo: 'bar' } };

    const result = filter.catch(exception, host);

    expect(result.message).toBe('fallback message');
    expect(result.extensions).toMatchObject({
      statusCode: 400,
      code: 'BAD_REQUEST',
    });
  });

  it('uses exception message when HttpException response object has no message', () => {
    const exception = new HttpException({ reason: 'teapot' }, 418);

    const result = filter.catch(exception, host);

    expect(result.message).toBe(exception.message);
    expect(result.extensions).toMatchObject({ statusCode: 418, success: false });
    expect(loggerSpy).toHaveBeenCalledWith(
      `GraphQL Exception: ${exception.message}`,
      exception.stack,
    );
  });

  it('handles HttpException with string response', () => {
    const exception = new HttpException('plain text', 500);

    const result = filter.catch(exception, host);

    expect(result.message).toBe('plain text');
    expect(result.extensions).toMatchObject({
      statusCode: 500,
      error: 'Unknown error',
    });
  });

  it('falls back to default message when nothing is provided', () => {
    const result = filter.catch({}, host);

    expect(result.message).toBe('An unexpected error occurred');
    expect(result.extensions).toMatchObject({ statusCode: 500, success: false });
  });

  it('handles non-object exception', () => {
    const result = filter.catch('string failure', host);

    expect(result.message).toBe('An unexpected error occurred');
    expect(result.extensions).toMatchObject({ statusCode: 500 });
    expect(loggerSpy).toHaveBeenCalledWith('GraphQL Exception', JSON.stringify('string failure'));
  });
});
