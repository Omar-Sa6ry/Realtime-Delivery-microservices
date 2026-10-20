import { Test, TestingModule } from '@nestjs/testing';
import { RedisSessionRepository } from './redis-session.repository';
import { RedisService } from '@bts-soft/core';
import { UserSessionData } from './redis-session.repository';

describe('RedisSessionRepository', () => {
  let service: RedisSessionRepository;
  let redisService: jest.Mocked<RedisService>;

  const mockSessionData: UserSessionData = {
    userId: 'user-123',
    email: 'test@example.com',
    role: 'USER',
    sessionId: 'session-123',
    createdAt: new Date('2024-01-01'),
  };

  beforeEach(async () => {
    redisService = {
      set: jest.fn().mockResolvedValue(undefined),
      get: jest.fn(),
      del: jest.fn().mockResolvedValue(undefined),
      keys: jest.fn(),
    } as unknown as jest.Mocked<RedisService>;

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        RedisSessionRepository,
        { provide: RedisService, useValue: redisService },
      ],
    }).compile();

    service = module.get<RedisSessionRepository>(RedisSessionRepository);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('createSession', () => {
    it('should store session data with TTL', async () => {
      await service.createSession('user-123', 'session-123', mockSessionData, 3600);

      expect(redisService.set).toHaveBeenCalledWith(
        'user:session:user-123:session-123',
        mockSessionData,
        3600,
      );
    });
  });

  describe('getSession', () => {
    it('should return session data if found', async () => {
      redisService.get.mockResolvedValue(mockSessionData);

      const result = await service.getSession('user-123', 'session-123');

      expect(result).toEqual(mockSessionData);
      expect(redisService.get).toHaveBeenCalledWith('user:session:user-123:session-123');
    });

    it('should return null if session not found', async () => {
      redisService.get.mockResolvedValue(null);

      const result = await service.getSession('user-123', 'session-123');

      expect(result).toBeNull();
    });
  });

  describe('revokeSession', () => {
    it('should delete session key', async () => {
      await service.revokeSession('user-123', 'session-123');

      expect(redisService.del).toHaveBeenCalledWith('user:session:user-123:session-123');
    });
  });

  describe('revokeAllUserSessions', () => {
    it('should delete all sessions for user', async () => {
      const keys = [
        'user:session:user-123:session-1',
        'user:session:user-123:session-2',
      ];
      (redisService as any).keys.mockResolvedValue(keys);

      await service.revokeAllUserSessions('user-123');

      expect((redisService as any).keys).toHaveBeenCalledWith('user:session:user-123:*');
      expect(redisService.del).toHaveBeenCalledTimes(2);
      expect(redisService.del).toHaveBeenCalledWith(keys[0]);
      expect(redisService.del).toHaveBeenCalledWith(keys[1]);
    });

    it('should handle no sessions gracefully', async () => {
      (redisService as any).keys.mockResolvedValue([]);

      await service.revokeAllUserSessions('user-123');

      expect(redisService.del).not.toHaveBeenCalled();
    });

    it('should handle null keys gracefully', async () => {
      (redisService as any).keys.mockResolvedValue(null);

      await service.revokeAllUserSessions('user-123');

      expect(redisService.del).not.toHaveBeenCalled();
    });
  });

  describe('storeRefreshToken', () => {
    it('should store refresh token hash with userId', async () => {
      await service.storeRefreshToken('token-hash-123', 'user-123', 604800);

      expect(redisService.set).toHaveBeenCalledWith(
        'user:refresh:token-hash-123',
        { userId: 'user-123' },
        604800,
      );
    });
  });

  describe('getUserIdByRefreshToken', () => {
    it('should return userId if refresh token found', async () => {
      redisService.get.mockResolvedValue({ userId: 'user-123' });

      const result = await service.getUserIdByRefreshToken('token-hash-123');

      expect(result).toBe('user-123');
      expect(redisService.get).toHaveBeenCalledWith('user:refresh:token-hash-123');
    });

    it('should return null if refresh token not found', async () => {
      redisService.get.mockResolvedValue(null);

      const result = await service.getUserIdByRefreshToken('token-hash-123');

      expect(result).toBeNull();
    });

    it('should return null if data has no userId', async () => {
      redisService.get.mockResolvedValue({});

      const result = await service.getUserIdByRefreshToken('token-hash-123');

      expect(result).toBeNull();
    });
  });

  describe('revokeRefreshToken', () => {
    it('should delete refresh token key', async () => {
      await service.revokeRefreshToken('token-hash-123');

      expect(redisService.del).toHaveBeenCalledWith('user:refresh:token-hash-123');
    });
  });
});