import { Test, TestingModule } from '@nestjs/testing';
import { JwtTokenProvider } from './jwt-token.provider';
import { JwtService } from '@nestjs/jwt';
import { ConfigService } from '@nestjs/config';
import { TokenPayload, TokenResult } from './jwt-token.provider';

describe('JwtTokenProvider', () => {
  let service: JwtTokenProvider;
  let jwtService: jest.Mocked<JwtService>;
  let configService: jest.Mocked<ConfigService>;

  const mockPayload: TokenPayload = {
    userId: 'user-123',
    email: 'test@example.com',
    role: 'USER',
    permissions: ['read:users'],
    sessionId: 'session-123',
  };

  const mockTokens: TokenResult = {
    accessToken: 'access-token-123',
    refreshToken: 'refresh-token-123',
    expiresIn: 86400,
  };

  beforeEach(async () => {
    jwtService = {
      signAsync: jest.fn(),
      verifyAsync: jest.fn(),
    } as unknown as jest.Mocked<JwtService>;

    configService = {
      get: jest.fn().mockReturnValue('test-secret'),
    } as unknown as jest.Mocked<ConfigService>;

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        JwtTokenProvider,
        { provide: JwtService, useValue: jwtService },
        { provide: ConfigService, useValue: configService },
      ],
    }).compile();

    service = module.get<JwtTokenProvider>(JwtTokenProvider);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('generateTokens', () => {
    it('should generate access and refresh tokens', async () => {
      jwtService.signAsync
        .mockResolvedValueOnce('access-token-123')
        .mockResolvedValueOnce('refresh-token-123');

      const result = await service.generateTokens(mockPayload);

      expect(result).toEqual(mockTokens);
      expect(jwtService.signAsync).toHaveBeenCalledTimes(2);
    });

    it('should use JWT_SECRET from config or default', async () => {
      jwtService.signAsync
        .mockResolvedValueOnce('access-token')
        .mockResolvedValueOnce('refresh-token');

      await service.generateTokens(mockPayload);

      const firstCall = jwtService.signAsync.mock.calls[0];
      const secondCall = jwtService.signAsync.mock.calls[1];

      expect(firstCall[1]).toEqual(expect.objectContaining({ secret: 'test-secret' }));
      expect(secondCall[1]).toEqual(expect.objectContaining({ secret: 'test-secret' }));
    });

    it('should use default secret if not in config', async () => {
      configService.get.mockReturnValue(undefined);
      jwtService.signAsync
        .mockResolvedValueOnce('access-token')
        .mockResolvedValueOnce('refresh-token');

      await service.generateTokens(mockPayload);

      const firstCall = jwtService.signAsync.mock.calls[0];
      expect(firstCall[1]).toEqual(expect.objectContaining({ secret: 'default_secret' }));
    });

    it('should include correct payload in access token', async () => {
      jwtService.signAsync
        .mockResolvedValueOnce('access-token')
        .mockResolvedValueOnce('refresh-token');

      await service.generateTokens(mockPayload);

      const accessTokenCall = jwtService.signAsync.mock.calls[0];
      expect(accessTokenCall[0]).toEqual(
        expect.objectContaining({
          sub: 'user-123',
          userId: 'user-123',
          email: 'test@example.com',
          role: 'USER',
          permissions: ['read:users'],
          sessionId: 'session-123',
        }),
      );
      expect(accessTokenCall[1]).toEqual(
        expect.objectContaining({ expiresIn: 86400 }),
      );
    });

    it('should include correct payload in refresh token', async () => {
      jwtService.signAsync
        .mockResolvedValueOnce('access-token')
        .mockResolvedValueOnce('refresh-token');

      await service.generateTokens(mockPayload);

      const refreshTokenCall = jwtService.signAsync.mock.calls[1];
      expect(refreshTokenCall[0]).toEqual(
        expect.objectContaining({
          sub: 'user-123',
          userId: 'user-123',
          sessionId: 'session-123',
          type: 'refresh',
        }),
      );
      expect(refreshTokenCall[1]).toEqual(
        expect.objectContaining({ expiresIn: 604800 }),
      );
    });
  });

  describe('verifyAccessToken', () => {
    it('should return decoded payload for valid token', async () => {
      const decoded = {
        userId: 'user-123',
        email: 'test@example.com',
        role: 'USER',
        permissions: ['read:users'],
        sessionId: 'session-123',
      };
      jwtService.verifyAsync.mockResolvedValue(decoded);

      const result = await service.verifyAccessToken('valid-token');

      expect(result).toEqual({
        userId: 'user-123',
        email: 'test@example.com',
        role: 'USER',
        permissions: ['read:users'],
        sessionId: 'session-123',
      });
      expect(jwtService.verifyAsync).toHaveBeenCalledWith('valid-token', {
        secret: 'test-secret',
      });
    });

    it('should return null for invalid token', async () => {
      jwtService.verifyAsync.mockRejectedValue(new Error('Invalid token'));

      const result = await service.verifyAccessToken('invalid-token');

      expect(result).toBeNull();
    });

    it('should use default secret if not in config', async () => {
      configService.get.mockReturnValue(undefined);
      jwtService.verifyAsync.mockResolvedValue({ sub: 'user-123' });

      await service.verifyAccessToken('token');

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('token', {
        secret: 'default_secret',
      });
    });

    it('should fallback to sub if userId not present', async () => {
      jwtService.verifyAsync.mockResolvedValue({ sub: 'user-456', email: 'test@example.com' });

      const result = await service.verifyAccessToken('token');

      expect(result?.userId).toBe('user-456');
    });
  });

  describe('verifyRefreshToken', () => {
    it('should return decoded payload for valid refresh token', async () => {
      const decoded = {
        userId: 'user-123',
        sessionId: 'session-123',
        type: 'refresh',
      };
      jwtService.verifyAsync.mockResolvedValue(decoded);

      const result = await service.verifyRefreshToken('valid-refresh-token');

      expect(result).toEqual({
        userId: 'user-123',
        email: '',
        role: '',
        permissions: [],
        sessionId: 'session-123',
      });
    });

    it('should return null if token type is not refresh', async () => {
      jwtService.verifyAsync.mockResolvedValue({ type: 'access' });

      const result = await service.verifyRefreshToken('access-token');

      expect(result).toBeNull();
    });

    it('should return null for invalid token', async () => {
      jwtService.verifyAsync.mockRejectedValue(new Error('Invalid token'));

      const result = await service.verifyRefreshToken('invalid-token');

      expect(result).toBeNull();
    });

    it('should use default secret if not in config', async () => {
      configService.get.mockReturnValue(undefined);
      jwtService.verifyAsync.mockResolvedValue({ sub: 'user-123', type: 'refresh' });

      await service.verifyRefreshToken('token');

      expect(jwtService.verifyAsync).toHaveBeenCalledWith('token', {
        secret: 'default_secret',
      });
    });
  });
});