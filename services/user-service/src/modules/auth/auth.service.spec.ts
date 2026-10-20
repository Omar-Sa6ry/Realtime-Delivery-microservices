import { Test, TestingModule } from '@nestjs/testing';
import { BadRequestException, UnauthorizedException } from '@nestjs/common';
import { AuthService } from './auth.service';
import { Repository, DataSource, QueryRunner, EntityManager } from 'typeorm';
import { User } from '../../common/database/entities/user.entity';
import { Outbox } from '../../common/database/entities/outbox.entity';
import { BcryptPasswordHasher } from '../../common/security/bcrypt-password.hasher';
import { JwtTokenProvider } from '../../common/security/jwt-token.provider';
import { RedisSessionRepository } from '../../common/database/repositories/redis-session.repository';
import { OutboxWorkerService } from '../../common/messaging/outbox-worker.service';
import { NotificationService } from '@bts-soft/core';
import { KafkaService, Role, rolePermissionsMap, UserKafkaTopics } from '@delivery-micro/shard';
import { UserFactory } from './user.factory';
import { I18nService } from 'nestjs-i18n';
import { UserRabbitMQPublisher } from '../rabbitmq/rabbitmq.publisher';
import {
  RegisterInput,
  LoginInput,
  ForgetPasswordInput,
  ResetPasswordInput,
  RefreshTokenInput,
} from './dto/auth.types';

describe('AuthService', () => {
  let service: AuthService;
  let userRepo: jest.Mocked<Repository<User>>;
  let passwordHasher: jest.Mocked<BcryptPasswordHasher>;
  let tokenProvider: jest.Mocked<JwtTokenProvider>;
  let sessionRepo: jest.Mocked<RedisSessionRepository>;
  let dataSource: jest.Mocked<DataSource>;
  let queryRunner: jest.Mocked<QueryRunner>;
  let outboxWorkerService: jest.Mocked<OutboxWorkerService>;
  let notificationService: jest.Mocked<NotificationService>;
  let kafkaService: jest.Mocked<KafkaService>;
  let userFactory: jest.Mocked<UserFactory>;
  let i18n: jest.Mocked<I18nService>;
  let rabbitmqPublisher: jest.Mocked<UserRabbitMQPublisher>;

  const mockUser: User = {
    id: 'user-123',
    email: 'test@example.com',
    passwordHash: 'hashed-password',
    firstName: 'John',
    lastName: 'Doe',
    phoneNumber: '+1234567890',
    role: Role.USER,
    isActive: true,
    imageUrl: 'https://example.com/avatar.jpg',
    addresses: [],
    createdAt: new Date('2024-01-01'),
    updatedAt: new Date('2024-01-01'),
    resetToken: undefined,
    resetTokenExpiry: undefined,
  } as User;

  const mockTokens = {
    accessToken: 'access-token-123',
    refreshToken: 'refresh-token-123',
    expiresIn: 86400,
  };

  beforeEach(async () => {
    userRepo = {
      findOne: jest.fn(),
      save: jest.fn(),
      count: jest.fn(),
    } as unknown as jest.Mocked<Repository<User>>;

    passwordHasher = {
      hash: jest.fn(),
      compare: jest.fn(),
    } as unknown as jest.Mocked<BcryptPasswordHasher>;

    tokenProvider = {
      generateTokens: jest.fn().mockResolvedValue(mockTokens),
      verifyAccessToken: jest.fn(),
      verifyRefreshToken: jest.fn(),
    } as unknown as jest.Mocked<JwtTokenProvider>;

    sessionRepo = {
      createSession: jest.fn().mockResolvedValue(undefined),
      getSession: jest.fn(),
      revokeSession: jest.fn().mockResolvedValue(undefined),
      revokeAllUserSessions: jest.fn().mockResolvedValue(undefined),
      storeRefreshToken: jest.fn().mockResolvedValue(undefined),
      getUserIdByRefreshToken: jest.fn(),
      revokeRefreshToken: jest.fn().mockResolvedValue(undefined),
    } as unknown as jest.Mocked<RedisSessionRepository>;

    queryRunner = {
      connect: jest.fn(),
      startTransaction: jest.fn(),
      commitTransaction: jest.fn(),
      rollbackTransaction: jest.fn(),
      release: jest.fn(),
      manager: {
        save: jest.fn(),
      } as unknown as EntityManager,
    } as unknown as jest.Mocked<QueryRunner>;

    dataSource = {
      createQueryRunner: jest.fn().mockReturnValue(queryRunner),
    } as unknown as jest.Mocked<DataSource>;

    outboxWorkerService = {
      enqueueEvent: jest.fn().mockResolvedValue(undefined),
    } as unknown as jest.Mocked<OutboxWorkerService>;

    const mockSend = jest.fn().mockResolvedValue(undefined);
    mockSend.mockReturnValue(Promise.resolve(undefined));

    notificationService = {
      send: mockSend,
    } as unknown as jest.Mocked<NotificationService>;

    kafkaService = {
      emit: jest.fn().mockReturnValue(Promise.resolve(undefined)),
    } as unknown as jest.Mocked<KafkaService>;

    userFactory = {
      createUser: jest.fn().mockReturnValue(mockUser),
    } as unknown as jest.Mocked<UserFactory>;

    i18n = {
      t: jest.fn((key: string) => key),
    } as unknown as jest.Mocked<I18nService>;

    rabbitmqPublisher = {
      publishUserCreated: jest.fn().mockResolvedValue(undefined),
      publishUserUpdated: jest.fn().mockResolvedValue(undefined),
      publishUserDeleted: jest.fn().mockResolvedValue(undefined),
    } as unknown as jest.Mocked<UserRabbitMQPublisher>;

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        AuthService,
        { provide: 'UserRepository', useValue: userRepo },
        { provide: BcryptPasswordHasher, useValue: passwordHasher },
        { provide: JwtTokenProvider, useValue: tokenProvider },
        { provide: RedisSessionRepository, useValue: sessionRepo },
        { provide: DataSource, useValue: dataSource },
        { provide: OutboxWorkerService, useValue: outboxWorkerService },
        { provide: NotificationService, useValue: notificationService },
        { provide: KafkaService, useValue: kafkaService },
        { provide: UserFactory, useValue: userFactory },
        { provide: I18nService, useValue: i18n },
        { provide: UserRabbitMQPublisher, useValue: rabbitmqPublisher },
      ],
    }).compile();

    service = module.get<AuthService>(AuthService);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('register', () => {
    const registerInput: RegisterInput = {
      email: 'test@example.com',
      password: 'password123',
      firstName: 'John',
      lastName: 'Doe',
      phoneNumber: '+1234567890',
      imageUrl: 'https://example.com/avatar.jpg',
    };

    beforeEach(() => {
      userRepo.findOne.mockResolvedValue(null);
      userRepo.count.mockResolvedValue(0);
      passwordHasher.hash.mockResolvedValue('hashed-password');
      queryRunner.manager.save.mockResolvedValue(mockUser);
      tokenProvider.generateTokens.mockResolvedValue(mockTokens);
    });

    it('should register new user successfully', async () => {
      const result = await service.register(registerInput);

      expect(result).toBeDefined();
      expect(result.user).toBeDefined();
      expect(result.accessToken).toBe('access-token-123');
      expect(result.refreshToken).toBe('refresh-token-123');
      expect(userRepo.findOne).toHaveBeenCalledTimes(2);
      expect(passwordHasher.hash).toHaveBeenCalledWith('password123');
      expect(userFactory.createUser).toHaveBeenCalled();
      expect(queryRunner.connect).toHaveBeenCalled();
      expect(queryRunner.startTransaction).toHaveBeenCalled();
      expect(queryRunner.commitTransaction).toHaveBeenCalled();
      expect(outboxWorkerService.enqueueEvent).toHaveBeenCalled();
      expect(kafkaService.emit).toHaveBeenCalledWith(
        UserKafkaTopics.USER_CREATED,
        UserKafkaTopics.USER_CREATED,
        expect.objectContaining({ userId: 'user-123' }),
      );
      expect(rabbitmqPublisher.publishUserCreated).toHaveBeenCalled();
      expect(sessionRepo.createSession).toHaveBeenCalled();
    });

    it('should assign ADMIN role to first user', async () => {
      userRepo.count.mockResolvedValue(0);

      await service.register(registerInput);

      expect(userFactory.createUser).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        expect.any(String),
        expect.any(String),
        Role.ADMIN,
        expect.any(String),
        expect.any(String),
      );
    });

    it('should assign USER role to subsequent users', async () => {
      userRepo.count.mockResolvedValue(5);

      await service.register(registerInput);

      expect(userFactory.createUser).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        expect.any(String),
        expect.any(String),
        Role.USER,
        expect.any(String),
        expect.any(String),
      );
    });

    it('should throw BadRequestException if email already exists', async () => {
      userRepo.findOne
        .mockResolvedValueOnce(mockUser)
        .mockResolvedValueOnce(null);

      await expect(service.register(registerInput)).rejects.toThrow(BadRequestException);
      // The code uses Promise.all, so either error could be thrown first
      expect(i18n.t).toHaveBeenCalled();
    });

    it('should throw BadRequestException if phone number already exists', async () => {
      userRepo.findOne
        .mockResolvedValueOnce(null)
        .mockResolvedValueOnce(mockUser);

      await expect(service.register(registerInput)).rejects.toThrow(BadRequestException);
      expect(i18n.t).toHaveBeenCalled();
    });

    it('should throw BadRequestException if both email and phone exist', async () => {
      userRepo.findOne
        .mockResolvedValueOnce(mockUser)
        .mockResolvedValueOnce(mockUser);

      await expect(service.register(registerInput)).rejects.toThrow(BadRequestException);
      expect(i18n.t).toHaveBeenCalled();
    });

    it('should rollback transaction on error', async () => {
      queryRunner.manager.save.mockRejectedValue(new Error('DB error'));

      await expect(service.register(registerInput)).rejects.toThrow('DB error');
      expect(queryRunner.rollbackTransaction).toHaveBeenCalled();
    });

    it('should not fail if notification or Kafka events fail', async () => {
      notificationService.send.mockRejectedValue(new Error('Email failed'));
      kafkaService.emit.mockRejectedValue(new Error('Kafka failed'));

      await expect(service.register(registerInput)).resolves.toBeDefined();
    });
  });

  describe('login', () => {
    const loginInput: LoginInput = {
      email: 'test@example.com',
      password: 'password123',
    };

    it('should login successfully with valid credentials', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      passwordHasher.compare.mockResolvedValue(true);
      tokenProvider.generateTokens.mockResolvedValue(mockTokens);

      const result = await service.login(loginInput);

      expect(result).toBeDefined();
      expect(result.user).toBeDefined();
      expect(result.accessToken).toBe('access-token-123');
      expect(result.refreshToken).toBe('refresh-token-123');
      expect(userRepo.findOne).toHaveBeenCalledWith({
        where: { email: 'test@example.com' },
      });
      expect(passwordHasher.compare).toHaveBeenCalledWith('password123', 'hashed-password');
      expect(tokenProvider.generateTokens).toHaveBeenCalled();
      expect(sessionRepo.createSession).toHaveBeenCalled();
    });

    it('should throw BadRequestException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.login(loginInput)).rejects.toThrow(BadRequestException);
      expect(i18n.t).toHaveBeenCalledWith('user.INVALID_CREDENTIALS');
    });

    it('should throw UnauthorizedException if password is invalid', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      passwordHasher.compare.mockResolvedValue(false);

      await expect(service.login(loginInput)).rejects.toThrow(UnauthorizedException);
      expect(i18n.t).toHaveBeenCalledWith('user.INVALID_CREDENTIALS');
    });
  });

  describe('forgetPassword', () => {
    const forgetPasswordInput: ForgetPasswordInput = {
      email: 'test@example.com',
    };

    it('should generate reset token and send email', async () => {
      const userWithToken = { ...mockUser, resetToken: '123456', resetTokenExpiry: new Date() };
      userRepo.findOne.mockResolvedValue(mockUser);
      userRepo.save.mockResolvedValue(userWithToken);

      await service.forgetPassword(forgetPasswordInput);

      expect(userRepo.findOne).toHaveBeenCalledWith({
        where: { email: 'test@example.com' },
      });
      expect(userRepo.save).toHaveBeenCalled();
      expect(notificationService.send).toHaveBeenCalledWith(
        'email',
        expect.objectContaining({
          recipientId: 'test@example.com',
          subject: 'Password Reset Code',
        }),
      );
      expect(kafkaService.emit).toHaveBeenCalledWith(
        UserKafkaTopics.PASSWORD_RESET_REQUESTED,
        UserKafkaTopics.PASSWORD_RESET_REQUESTED,
        expect.objectContaining({ userId: 'user-123' }),
      );
    });

    it('should not throw if user not found (security)', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.forgetPassword(forgetPasswordInput)).resolves.toBeUndefined();
      expect(userRepo.save).not.toHaveBeenCalled();
      expect(notificationService.send).not.toHaveBeenCalled();
    });

    it('should not throw if notification or Kafka fails', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      userRepo.save.mockResolvedValue(mockUser);
      notificationService.send.mockRejectedValue(new Error('Email failed'));
      kafkaService.emit.mockRejectedValue(new Error('Kafka failed'));

      await expect(service.forgetPassword(forgetPasswordInput)).resolves.toBeUndefined();
    });
  });

  describe('resetPassword', () => {
    const resetPasswordInput: ResetPasswordInput = {
      token: '123456',
      passwordNew: 'newPassword123',
    };

    it('should reset password with valid token', async () => {
      const userWithToken = {
        ...mockUser,
        resetToken: '123456',
        resetTokenExpiry: new Date(Date.now() + 60000),
      };
      userRepo.findOne.mockResolvedValue(userWithToken);
      passwordHasher.hash.mockResolvedValue('new-hashed-password');
      userRepo.save.mockResolvedValue({ ...userWithToken, passwordHash: 'new-hashed-password' });

      await service.resetPassword(resetPasswordInput);

      expect(userRepo.findOne).toHaveBeenCalledWith({
        where: { resetToken: '123456' },
      });
      expect(passwordHasher.hash).toHaveBeenCalledWith('newPassword123');
      expect(userRepo.save).toHaveBeenCalledWith(
        expect.objectContaining({
          passwordHash: 'new-hashed-password',
          resetToken: undefined,
          resetTokenExpiry: undefined,
        }),
      );
    });

    it('should throw BadRequestException if token invalid', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.resetPassword(resetPasswordInput)).rejects.toThrow(
        BadRequestException,
      );
      expect(i18n.t).toHaveBeenCalledWith('user.INVALID_RESET_TOKEN');
    });

    it('should throw BadRequestException if token expired', async () => {
      const userWithExpiredToken = {
        ...mockUser,
        resetToken: '123456',
        resetTokenExpiry: new Date(Date.now() - 60000),
      };
      userRepo.findOne.mockResolvedValue(userWithExpiredToken);

      await expect(service.resetPassword(resetPasswordInput)).rejects.toThrow(
        BadRequestException,
      );
    });
  });

  describe('logout', () => {
    it('should revoke session', async () => {
      await service.logout('user-123', 'session-123');

      expect(sessionRepo.revokeSession).toHaveBeenCalledWith('user-123', 'session-123');
    });
  });

  describe('refreshToken', () => {
    const refreshTokenInput: RefreshTokenInput = {
      refreshToken: 'refresh-token-123',
    };

    it('should refresh token successfully', async () => {
      const tokenPayload = {
        userId: 'user-123',
        email: 'test@example.com',
        role: Role.USER,
        permissions: rolePermissionsMap[Role.USER] || [],
        sessionId: 'session-123',
      };
      tokenProvider.verifyRefreshToken.mockResolvedValue(tokenPayload);
      sessionRepo.getSession.mockResolvedValue({
        userId: 'user-123',
        email: 'test@example.com',
        role: Role.USER,
        sessionId: 'session-123',
        createdAt: new Date(),
      });
      userRepo.findOne.mockResolvedValue(mockUser);
      tokenProvider.generateTokens.mockResolvedValue(mockTokens);

      const result = await service.refreshToken(refreshTokenInput);

      expect(result).toBeDefined();
      expect(result.accessToken).toBe('access-token-123');
      expect(result.refreshToken).toBe('refresh-token-123');
      expect(tokenProvider.verifyRefreshToken).toHaveBeenCalledWith('refresh-token-123');
      expect(sessionRepo.getSession).toHaveBeenCalledWith('user-123', 'session-123');
      expect(userRepo.findOne).toHaveBeenCalledWith({ where: { id: 'user-123' } });
      expect(tokenProvider.generateTokens).toHaveBeenCalled();
      expect(sessionRepo.createSession).toHaveBeenCalled();
    });

    it('should throw UnauthorizedException if refresh token invalid', async () => {
      tokenProvider.verifyRefreshToken.mockResolvedValue(null);

      await expect(service.refreshToken(refreshTokenInput)).rejects.toThrow(
        UnauthorizedException,
      );
      expect(i18n.t).toHaveBeenCalledWith('user.INVALID_TOKEN');
    });

    it('should throw UnauthorizedException if session not found', async () => {
      tokenProvider.verifyRefreshToken.mockResolvedValue({
        userId: 'user-123',
        sessionId: 'session-123',
      } as any);
      sessionRepo.getSession.mockResolvedValue(null);

      await expect(service.refreshToken(refreshTokenInput)).rejects.toThrow(
        UnauthorizedException,
      );
      expect(i18n.t).toHaveBeenCalledWith('user.SESSION_EXPIRED');
    });

    it('should throw UnauthorizedException if user not found', async () => {
      tokenProvider.verifyRefreshToken.mockResolvedValue({
        userId: 'user-123',
        sessionId: 'session-123',
      } as any);
      sessionRepo.getSession.mockResolvedValue({
        userId: 'user-123',
        email: 'test@example.com',
        role: Role.USER,
        sessionId: 'session-123',
        createdAt: new Date(),
      });
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.refreshToken(refreshTokenInput)).rejects.toThrow(
        UnauthorizedException,
      );
      expect(i18n.t).toHaveBeenCalledWith('user.NOT_FOUND');
    });
  });
});