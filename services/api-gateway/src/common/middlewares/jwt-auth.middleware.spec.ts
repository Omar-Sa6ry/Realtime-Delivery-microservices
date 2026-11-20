import { JwtAuthMiddleware } from './jwt-auth.middleware';

describe('JwtAuthMiddleware', () => {
  let middleware: JwtAuthMiddleware;
  let jwtService: { verifyAsync: jest.Mock };
  let req: any;
  let res: any;
  let next: jest.Mock;

  beforeEach(() => {
    jwtService = { verifyAsync: jest.fn() };
    middleware = new JwtAuthMiddleware(jwtService as any);
    req = { headers: {} };
    res = {};
    next = jest.fn();
    process.env.JWT_SECRET = 'unit-secret';
  });

  afterEach(() => {
    delete process.env.JWT_SECRET;
  });

  it('calls next() when there is no Authorization header', async () => {
    await middleware.use(req, res, next);

    expect(next).toHaveBeenCalledTimes(1);
    expect(jwtService.verifyAsync).not.toHaveBeenCalled();
    expect(req.user).toBeUndefined();
  });

  it('calls next() and skips verification for a non-Bearer header', async () => {
    req.headers.authorization = 'Basic abc123';

    await middleware.use(req, res, next);

    expect(next).toHaveBeenCalledTimes(1);
    expect(jwtService.verifyAsync).not.toHaveBeenCalled();
    expect(req.user).toBeUndefined();
  });

  describe('with a valid Bearer token', () => {
    it('attaches the decoded user and calls next()', async () => {
      req.headers.authorization = 'Bearer valid-token';
      jwtService.verifyAsync.mockResolvedValue({
        sub: 'user-1',
        role: 'DRIVER',
        sessionId: 'sess-1',
      });

      await middleware.use(req, res, next);

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('valid-token', {
        secret: 'unit-secret',
      });
      expect(req.user).toEqual({
        userId: 'user-1',
        role: 'DRIVER',
        sessionId: 'sess-1',
      });
      expect(next).toHaveBeenCalledTimes(1);
    });

    it('uses the JWT_SECRET env var for verification', async () => {
      process.env.JWT_SECRET = 'another-secret';
      req.headers.authorization = 'Bearer t';
      jwtService.verifyAsync.mockResolvedValue({ sub: 'u' });

      await middleware.use(req, res, next);

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('t', {
        secret: 'another-secret',
      });
    });

    it('falls back to the default secret when JWT_SECRET is unset', async () => {
      delete process.env.JWT_SECRET;
      req.headers.authorization = 'Bearer t';
      jwtService.verifyAsync.mockResolvedValue({ sub: 'u' });

      await middleware.use(req, res, next);

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('t', {
        secret: 'super-secret-jwt-key',
      });
    });

    it('maps payload.userId when sub is absent', async () => {
      req.headers.authorization = 'Bearer t';
      jwtService.verifyAsync.mockResolvedValue({ userId: 'u-by-userId' });

      await middleware.use(req, res, next);

      expect(req.user.userId).toBe('u-by-userId');
    });

    it('maps payload.id when both sub and userId are absent', async () => {
      req.headers.authorization = 'Bearer t';
      jwtService.verifyAsync.mockResolvedValue({ id: 'u-by-id' });

      await middleware.use(req, res, next);

      expect(req.user.userId).toBe('u-by-id');
    });

    it('defaults role to USER when role is absent', async () => {
      req.headers.authorization = 'Bearer t';
      jwtService.verifyAsync.mockResolvedValue({ sub: 'u' });

      await middleware.use(req, res, next);

      expect(req.user.role).toBe('USER');
    });

    it('leaves sessionId undefined when absent', async () => {
      req.headers.authorization = 'Bearer t';
      jwtService.verifyAsync.mockResolvedValue({ sub: 'u' });

      await middleware.use(req, res, next);

      expect(req.user.sessionId).toBeUndefined();
    });
  });

  describe('with an invalid token', () => {
    it('treats the request as anonymous and still calls next()', async () => {
      req.headers.authorization = 'Bearer bad';
      jwtService.verifyAsync.mockRejectedValue(new Error('expired'));

      await expect(middleware.use(req, res, next)).resolves.toBeUndefined();

      expect(req.user).toBeUndefined();
      expect(next).toHaveBeenCalledTimes(1);
    });

    it('does not throw even if verification rejects', async () => {
      req.headers.authorization = 'Bearer bad';
      jwtService.verifyAsync.mockRejectedValue(new Error('nope'));

      await expect(middleware.use(req, res, next)).resolves.not.toThrow();
    });
  });
});
