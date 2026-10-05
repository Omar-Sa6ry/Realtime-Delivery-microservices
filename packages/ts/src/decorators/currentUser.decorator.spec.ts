import { BadRequestException } from '@nestjs/common';
import type { ExecutionContext } from '@nestjs/common';
import { GqlExecutionContext } from '@nestjs/graphql';
import { CurrentUser } from './currentUser.decorator';
import { CurrentUserMsg } from '../constants/messages.constant';

jest.mock('@nestjs/graphql', () => ({
  __esModule: true,
  GqlExecutionContext: { create: jest.fn() },
}));

type ParamFactory = (data: unknown, context: ExecutionContext) => unknown;

describe('CurrentUser decorator', () => {
  const makeHttpContext = (): ExecutionContext =>
    ({
      getType: () => 'http',
      getHandler: () => () => undefined,
      getClass: () => class {},
      switchToHttp: () => ({
        getRequest: () => ({ user: undefined }),
        getResponse: () => ({}),
      }),
    }) as unknown as ExecutionContext;

  const makeGqlContext = (): ExecutionContext =>
    ({
      getType: () => 'graphql',
      getHandler: () => () => undefined,
      getClass: () => class {},
      getArgs: () => [{}, {}, {}, {}],
    }) as unknown as ExecutionContext;

  const applyDecorator = () => {
    class Target {
      handle(_user: unknown) {
        return undefined;
      }
    }
    CurrentUser()(Target.prototype, 'handle', 0);
    const metadata = Reflect.getMetadata('__routeArguments__', Target, 'handle');
    const entry = Object.values<any>(metadata)[0];
    return { Target, entry, factory: entry.factory as ParamFactory };
  };

  const mockContext = (context: ExecutionContext, ctxValue: any) => {
    (GqlExecutionContext.create as unknown as jest.Mock).mockReturnValue({
      getContext: () => ctxValue,
    });
  };

  beforeEach(() => {
    (GqlExecutionContext.create as unknown as jest.Mock).mockReset();
  });

  it('registers a parameter factory in route argument metadata', () => {
    const { entry } = applyDecorator();

    expect(typeof entry.factory).toBe('function');
    expect(entry.index).toBe(0);
    expect(entry.data).toBeUndefined();
  });

  it('extracts user from graphql request context', () => {
    const { factory } = applyDecorator();
    const user = { id: 'u1', role: 'admin' };
    const context = makeGqlContext();
    mockContext(context, { req: { user } });

    const result = factory(undefined, context);

    expect(result).toBe(user);
    expect(GqlExecutionContext.create).toHaveBeenCalledWith(context);
  });

  it('extracts user from http style execution context', () => {
    const { factory } = applyDecorator();
    const user = { id: 'u2', role: 'driver' };
    const context = makeHttpContext();
    mockContext(context, { req: { user } });

    expect(factory(undefined, context)).toBe(user);
  });

  it('throws BadRequestException when user is missing on request', () => {
    const { factory } = applyDecorator();
    const context = makeGqlContext();
    mockContext(context, { req: {} });

    expect(() => factory(undefined, context)).toThrow(BadRequestException);
    expect(() => factory(undefined, context)).toThrow(CurrentUserMsg);
  });

  it('throws BadRequestException when user is null', () => {
    const { factory } = applyDecorator();
    const context = makeGqlContext();
    mockContext(context, { req: { user: null } });

    expect(() => factory(undefined, context)).toThrow(CurrentUserMsg);
  });
});
