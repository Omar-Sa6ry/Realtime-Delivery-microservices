import { Logger } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import type { IncomingMessage } from 'http';
import { WS_JWT_SERVICE, WsJwtStrategy } from './ws-jwt.strategy';
import { WsErrorCode, WsException } from './ws-errors';

const makeRequest = (
  overrides: { headers?: Record<string, string>; url?: string } = {},
): IncomingMessage =>
  ({
    headers: {},
    url: '/ws',
    ...overrides,
  }) as unknown as IncomingMessage;

describe('WsJwtStrategy', () => {
  let jwtService: { verifyAsync: jest.Mock };
  let strategy: WsJwtStrategy;
  let warnSpy: jest.SpyInstance;

  beforeEach(() => {
    jwtService = { verifyAsync: jest.fn() };
    strategy = new WsJwtStrategy(jwtService);
    warnSpy = jest.spyOn(Logger.prototype, 'warn').mockImplementation();
  });

  afterEach(() => {
    warnSpy.mockRestore();
  });

  it('is injectable through the WS_JWT_SERVICE token', async () => {
    const module = await Test.createTestingModule({
      providers: [
        WsJwtStrategy,
        { provide: WS_JWT_SERVICE, useValue: jwtService },
      ],
    }).compile();

    expect(module.get(WsJwtStrategy)).toBeInstanceOf(WsJwtStrategy);
  });

  describe('token extraction', () => {
    it('rejects when neither a header nor a query token exists', async () => {
      const error = await strategy
        .authenticate(makeRequest({ url: '/ws' }))
        .catch((err) => err);

      expect(error).toBeInstanceOf(WsException);
      expect(error.code).toBe(WsErrorCode.UNAUTHENTICATED);
      expect(error.message).toBe('Missing token');
      expect(error.retryable).toBe(false);
      expect(jwtService.verifyAsync).not.toHaveBeenCalled();
    });

    it('rejects when the authorization header is not a bearer token', async () => {
      const error = await strategy
        .authenticate(
          makeRequest({ headers: { authorization: 'Basic dXNlcjpwdw==' } }),
        )
        .catch((err) => err);

      expect(error).toBeInstanceOf(WsException);
      expect(error.message).toBe('Missing token');
      expect(jwtService.verifyAsync).not.toHaveBeenCalled();
    });

    it('rejects when the bearer prefix casing does not match', async () => {
      const error = await strategy
        .authenticate(
          makeRequest({ headers: { authorization: 'bearer lowercase' } }),
        )
        .catch((err) => err);

      expect(error).toBeInstanceOf(WsException);
      expect(error.message).toBe('Missing token');
    });

    it('reads the token from the bearer header', async () => {
      jwtService.verifyAsync.mockResolvedValue({ userId: 'u-1', role: 'DRIVER' });

      const payload = await strategy.authenticate(
        makeRequest({ headers: { authorization: 'Bearer header-token' } }),
      );

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('header-token');
      expect(payload.userId).toBe('u-1');
    });

    it('reads a token that follows other query params', async () => {
      jwtService.verifyAsync.mockResolvedValue({ userId: 'u-2' });

      await strategy.authenticate(
        makeRequest({ url: '/ws?foo=bar&token=query-token' }),
      );

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('query-token');
    });

    it('url-decodes the query token', async () => {
      jwtService.verifyAsync.mockResolvedValue({ userId: 'u-3' });

      await strategy.authenticate(
        makeRequest({ url: '/ws?token=a%2Fb%20c' }),
      );

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('a/b c');
    });

    it('stops the query token at the first ampersand', async () => {
      jwtService.verifyAsync.mockResolvedValue({ userId: 'u-4' });

      await strategy.authenticate(
        makeRequest({ url: '/ws?token=first&other=second' }),
      );

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('first');
    });

    it('ignores an empty query value', async () => {
      const error = await strategy
        .authenticate(makeRequest({ url: '/ws?token=' }))
        .catch((err) => err);

      expect(error).toBeInstanceOf(WsException);
      expect(error.message).toBe('Missing token');
      expect(jwtService.verifyAsync).not.toHaveBeenCalled();
    });

    it('handles a request without a url', async () => {
      const error = await strategy
        .authenticate({ headers: {}, url: undefined } as unknown as IncomingMessage)
        .catch((err) => err);

      expect(error).toBeInstanceOf(WsException);
      expect(error.message).toBe('Missing token');
      expect(jwtService.verifyAsync).not.toHaveBeenCalled();
    });
  });

  describe('verification', () => {
    it('returns the normalized payload on success', async () => {
      jwtService.verifyAsync.mockResolvedValue({
        userId: 'u-5',
        sub: 'sub-5',
        id: 'id-5',
        role: 'DRIVER',
        email: 'driver@x.com',
        sessionId: 'sess-5',
        iat: 1,
        exp: 2,
        tenant: 'ignored',
      });

      const payload = await strategy.authenticate(
        makeRequest({ headers: { authorization: 'Bearer t' } }),
      );

      expect(payload).toEqual({
        userId: 'u-5',
        sub: 'sub-5',
        id: 'id-5',
        role: 'DRIVER',
        email: 'driver@x.com',
        sessionId: 'sess-5',
        iat: 1,
        exp: 2,
      });
      expect(payload).not.toHaveProperty('tenant');
      expect(warnSpy).not.toHaveBeenCalled();
    });

    it('falls back to sub when userId is missing', async () => {
      jwtService.verifyAsync.mockResolvedValue({ sub: 'sub-only', role: 'DRIVER' });

      const payload = await strategy.authenticate(
        makeRequest({ headers: { authorization: 'Bearer t' } }),
      );

      expect(payload.userId).toBe('sub-only');
      expect(payload.id).toBeUndefined();
    });

    it('falls back to id when userId and sub are missing', async () => {
      jwtService.verifyAsync.mockResolvedValue({ id: 'id-only', role: 'DRIVER' });

      const payload = await strategy.authenticate(
        makeRequest({ headers: { authorization: 'Bearer t' } }),
      );

      expect(payload.userId).toBe('id-only');
      expect(payload.sub).toBeUndefined();
    });

    it('leaves userId undefined when the payload has no identity fields', async () => {
      jwtService.verifyAsync.mockResolvedValue({ role: 'DRIVER' });

      const payload = await strategy.authenticate(
        makeRequest({ headers: { authorization: 'Bearer t' } }),
      );

      expect(payload.userId).toBeUndefined();
      expect(payload.role).toBe('DRIVER');
      expect(payload.email).toBeUndefined();
      expect(payload.sessionId).toBeUndefined();
      expect(payload.iat).toBeUndefined();
      expect(payload.exp).toBeUndefined();
    });

    it('treats an expired token as unauthenticated', async () => {
      jwtService.verifyAsync.mockRejectedValue(new Error('jwt expired'));

      const error = await strategy
        .authenticate(makeRequest({ headers: { authorization: 'Bearer t' } }))
        .catch((err) => err);

      expect(error).toBeInstanceOf(WsException);
      expect(error.code).toBe(WsErrorCode.UNAUTHENTICATED);
      expect(error.message).toBe('Invalid or expired token');
      expect(error.retryable).toBe(false);
      expect(warnSpy).toHaveBeenCalledWith(
        'WebSocket JWT verification failed: jwt expired',
      );
    });

    it('treats a malformed token as unauthenticated', async () => {
      jwtService.verifyAsync.mockRejectedValue('not-an-error-object');

      const error = await strategy
        .authenticate(makeRequest({ headers: { authorization: 'Bearer t' } }))
        .catch((err) => err);

      expect(error).toBeInstanceOf(WsException);
      expect(error.message).toBe('Invalid or expired token');
      expect(warnSpy).toHaveBeenCalledWith(
        'WebSocket JWT verification failed: undefined',
      );
    });

    it('propagates a token decoding error unwrapped', async () => {
      await expect(
        strategy.authenticate(makeRequest({ url: '/ws?token=%E0%A4%A' })),
      ).rejects.toThrow(URIError);
      expect(jwtService.verifyAsync).not.toHaveBeenCalled();
      expect(warnSpy).not.toHaveBeenCalled();
    });
  });
});
