import { ExecutionContext, UnauthorizedException } from '@nestjs/common';
import { GqlExecutionContext } from '@nestjs/graphql';
import { JwtAuthGuard } from './auth.guard';

describe('JwtAuthGuard', () => {
  let guard: JwtAuthGuard;
  let jwtService: { verifyAsync: jest.Mock };

  beforeEach(() => {
    jwtService = { verifyAsync: jest.fn() };
    guard = new JwtAuthGuard(jwtService as any);
    process.env.JWT_SECRET = 'guard-secret';
  });

  afterEach(() => {
    delete process.env.JWT_SECRET;
  });

  /** Builds a fake HTTP ExecutionContext. */
  function httpContext(req: any): ExecutionContext {
    return {
      getType: () => 'http',
      switchToHttp: () => ({ getRequest: () => req }),
    } as unknown as ExecutionContext;
  }

  /** Builds a fake GraphQL ExecutionContext. */
  function gqlContext(req: any): ExecutionContext {
    return {
      getType: () => 'graphql',
      getArgs: () => [null, null, { req }, null],
      getClass: () => class {},
      getHandler: () => () => undefined,
    } as unknown as ExecutionContext;
  }

  describe('missing / non-Bearer header', () => {
    it('allows a request with no Authorization header (anonymous)', async () => {
      const req: any = { headers: {} };

      await expect(guard.canActivate(httpContext(req))).resolves.toBe(true);
      expect(jwtService.verifyAsync).not.toHaveBeenCalled();
      expect(req.user).toBeUndefined();
    });

    it('allows a non-Bearer Authorization header', async () => {
      const req = { headers: { authorization: 'Basic xyz' } };

      await expect(guard.canActivate(httpContext(req))).resolves.toBe(true);
      expect(jwtService.verifyAsync).not.toHaveBeenCalled();
    });
  });

  describe('valid Bearer token (HTTP context)', () => {
    it('attaches the user and returns true', async () => {
      const req: any = { headers: { authorization: 'Bearer tok' } };
      jwtService.verifyAsync.mockResolvedValue({
        sub: 'u1',
        role: 'ADMIN',
        sessionId: 's1',
      });

      await expect(guard.canActivate(httpContext(req))).resolves.toBe(true);
      expect(req.user).toEqual({
        userId: 'u1',
        role: 'ADMIN',
        sessionId: 's1',
      });
    });

    it('verifies using JWT_SECRET', async () => {
      const req: any = { headers: { authorization: 'Bearer tok' } };
      jwtService.verifyAsync.mockResolvedValue({ sub: 'u1' });

      await guard.canActivate(httpContext(req));

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('tok', {
        secret: 'guard-secret',
      });
    });

    it('falls back to the default secret when JWT_SECRET is unset', async () => {
      delete process.env.JWT_SECRET;
      const req: any = { headers: { authorization: 'Bearer tok' } };
      jwtService.verifyAsync.mockResolvedValue({ sub: 'u1' });

      await guard.canActivate(httpContext(req));

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('tok', {
        secret: 'super-secret-jwt-key',
      });
    });

    it('maps userId from payload.userId fallback', async () => {
      const req: any = { headers: { authorization: 'Bearer tok' } };
      jwtService.verifyAsync.mockResolvedValue({ userId: 'fallback' });

      await guard.canActivate(httpContext(req));

      expect(req.user.userId).toBe('fallback');
    });

    it('maps userId from payload.id fallback', async () => {
      const req: any = { headers: { authorization: 'Bearer tok' } };
      jwtService.verifyAsync.mockResolvedValue({ id: 'byId' });

      await guard.canActivate(httpContext(req));

      expect(req.user.userId).toBe('byId');
    });

    it('defaults role to USER', async () => {
      const req: any = { headers: { authorization: 'Bearer tok' } };
      jwtService.verifyAsync.mockResolvedValue({ sub: 'u1' });

      await guard.canActivate(httpContext(req));

      expect(req.user.role).toBe('USER');
    });
  });

  describe('GraphQL execution context', () => {
    it('reads the request from the GraphQL context', async () => {
      const req: any = { headers: { authorization: 'Bearer gt' } };
      jwtService.verifyAsync.mockResolvedValue({
        sub: 'g1',
        role: 'DRIVER',
        sessionId: 'gs',
      });

      await expect(guard.canActivate(gqlContext(req))).resolves.toBe(true);
      expect(req.user).toEqual({
        userId: 'g1',
        role: 'DRIVER',
        sessionId: 'gs',
      });
    });

    it('allows anonymous GraphQL requests', async () => {
      const req: any = { headers: {} };

      await expect(guard.canActivate(gqlContext(req))).resolves.toBe(true);
    });
  });

  describe('invalid token', () => {
    it('throws UnauthorizedException for an HTTP request', async () => {
      const req: any = { headers: { authorization: 'Bearer bad' } };
      jwtService.verifyAsync.mockRejectedValue(new Error('expired'));

      await expect(guard.canActivate(httpContext(req))).rejects.toThrow(
        UnauthorizedException,
      );
      await expect(guard.canActivate(httpContext(req))).rejects.toThrow(
        'Invalid or expired JWT token',
      );
    });

    it('throws UnauthorizedException for a GraphQL request', async () => {
      const req: any = { headers: { authorization: 'Bearer bad' } };
      jwtService.verifyAsync.mockRejectedValue(new Error('bad'));

      await expect(guard.canActivate(gqlContext(req))).rejects.toThrow(
        UnauthorizedException,
      );
    });
  });

  it('uses GqlExecutionContext for graphql-typed contexts', async () => {
    const createSpy = jest.spyOn(GqlExecutionContext, 'create');
    const req: any = { headers: { authorization: 'Bearer gt' } };
    jwtService.verifyAsync.mockResolvedValue({ sub: 'g' });

    await guard.canActivate(gqlContext(req));

    expect(createSpy).toHaveBeenCalled();
    createSpy.mockRestore();
  });
});
