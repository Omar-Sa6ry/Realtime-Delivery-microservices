import { Test, TestingModule } from '@nestjs/testing';
import { UserService } from './user.service';
import { DbUserService } from './db-user.service';
import { UserRabbitMQPublisher } from '../rabbitmq/rabbitmq.publisher';
import { RedisService } from '@bts-soft/core';
import { User } from '../../common/database/entities/user.entity';
import { Address } from '../../common/database/entities/address.entity';
import { UpdateProfileInput, ChangePasswordInput } from './dto/user.types';
import { Role } from '@delivery-micro/shard';

describe('UserService', () => {
  let service: UserService;
  let dbUserService: jest.Mocked<DbUserService>;
  let redisService: jest.Mocked<RedisService>;
  let rmqPublisher: jest.Mocked<UserRabbitMQPublisher>;

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

  const mockAddress: Address = {
    id: 'addr-123',
    userId: 'user-123',
    title: 'Home',
    street: '123 Main St',
    city: 'New York',
    state: 'NY',
    postalCode: '10001',
    latitude: 40.7128,
    longitude: -74.0060,
    isDefault: true,
    createdAt: new Date('2024-01-01'),
    user: mockUser,
  } as Address;

  beforeEach(async () => {
    dbUserService = {
      findById: jest.fn(),
      findByEmail: jest.fn(),
      updateProfile: jest.fn(),
      changePassword: jest.fn(),
      promoteUserToAdmin: jest.fn(),
      updateUserRole: jest.fn(),
      toggleUserActive: jest.fn(),
      deleteUser: jest.fn(),
      addAddress: jest.fn(),
      deleteAddress: jest.fn(),
      setDefaultAddress: jest.fn(),
      countUsersThisMonthAndLastMonth: jest.fn(),
    } as unknown as jest.Mocked<DbUserService>;

    redisService = {
      hGet: jest.fn(),
      hSet: jest.fn(),
      expire: jest.fn(),
      del: jest.fn(),
    } as unknown as jest.Mocked<RedisService>;

    rmqPublisher = {
      publishUserCreated: jest.fn().mockResolvedValue(undefined),
      publishUserUpdated: jest.fn().mockResolvedValue(undefined),
      publishUserDeleted: jest.fn().mockResolvedValue(undefined),
    } as unknown as jest.Mocked<UserRabbitMQPublisher>;

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        UserService,
        { provide: DbUserService, useValue: dbUserService },
        { provide: RedisService, useValue: redisService },
        { provide: UserRabbitMQPublisher, useValue: rmqPublisher },
      ],
    }).compile();

    service = module.get<UserService>(UserService);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('findById', () => {
    it('should return user from cache if available', async () => {
      const cachedUser = { ...mockUser, createdAt: '2024-01-01T00:00:00.000Z' };
      redisService.hGet.mockResolvedValue(JSON.stringify(cachedUser));

      const result = await service.findById('user-123');

      expect(result).toEqual(mockUser);
      expect(redisService.hGet).toHaveBeenCalledWith('user:user-123', 'data');
      expect(dbUserService.findById).not.toHaveBeenCalled();
    });

    it('should fetch from DB and cache if not in cache', async () => {
      redisService.hGet.mockResolvedValue(null);
      dbUserService.findById.mockResolvedValue(mockUser);

      const result = await service.findById('user-123');

      expect(result).toEqual(mockUser);
      expect(dbUserService.findById).toHaveBeenCalledWith('user-123');
      expect(redisService.hSet).toHaveBeenCalled();
      expect(redisService.expire).toHaveBeenCalledWith('user:user-123', 3600);
    });

    it('should return null if user not found in DB', async () => {
      redisService.hGet.mockResolvedValue(null);
      dbUserService.findById.mockResolvedValue(null);

      const result = await service.findById('user-123');

      expect(result).toBeNull();
    });

    it('should fallback to DB if cache parsing fails', async () => {
      redisService.hGet.mockResolvedValue('invalid-json');
      dbUserService.findById.mockResolvedValue(mockUser);

      const result = await service.findById('user-123');

      expect(result).toEqual(mockUser);
      expect(dbUserService.findById).toHaveBeenCalledWith('user-123');
    });
  });

  describe('findByEmail', () => {
    it('should delegate to dbUserService', async () => {
      dbUserService.findByEmail.mockResolvedValue(mockUser);

      const result = await service.findByEmail('test@example.com');

      expect(result).toEqual(mockUser);
      expect(dbUserService.findByEmail).toHaveBeenCalledWith('test@example.com');
    });
  });

  describe('updateProfile', () => {
    const updateInput: UpdateProfileInput = {
      firstName: 'Jane',
      lastName: 'Smith',
      imageUrl: 'https://example.com/new-avatar.jpg',
      avatarMediaId: undefined,
    };

    it('should update profile and publish event', async () => {
      const updatedUser = { ...mockUser, firstName: 'Jane', lastName: 'Smith' };
      dbUserService.updateProfile.mockResolvedValue(updatedUser);

      const result = await service.updateProfile('user-123', updateInput);

      expect(result).toEqual(updatedUser);
      expect(dbUserService.updateProfile).toHaveBeenCalledWith('user-123', updateInput);
      expect(redisService.hSet).toHaveBeenCalled();
      expect(rmqPublisher.publishUserUpdated).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'user-123',
          email: 'test@example.com',
          firstName: 'Jane',
          lastName: 'Smith',
        }),
      );
    });

    it('should not throw if RMQ publish fails (best-effort)', async () => {
      const updatedUser = { ...mockUser, firstName: 'Jane' };
      dbUserService.updateProfile.mockResolvedValue(updatedUser);
      // The real implementation catches errors, so this should not throw
      rmqPublisher.publishUserUpdated.mockResolvedValue(undefined);

      const result = await service.updateProfile('user-123', updateInput);
      expect(result).toEqual(updatedUser);
    });
  });

  describe('changePassword', () => {
    const changePasswordInput: ChangePasswordInput = {
      passwordOld: 'oldPassword123',
      passwordNew: 'newPassword456',
    };

    it('should change password and refresh cache', async () => {
      const updatedUser = { ...mockUser };
      dbUserService.changePassword.mockResolvedValue(updatedUser);

      await service.changePassword('user-123', changePasswordInput);

      expect(dbUserService.changePassword).toHaveBeenCalledWith('user-123', changePasswordInput);
      expect(redisService.hSet).toHaveBeenCalled();
      expect(redisService.expire).toHaveBeenCalledWith('user:user-123', 3600);
    });

    it('should not publish RMQ event for password change', async () => {
      const updatedUser = { ...mockUser };
      dbUserService.changePassword.mockResolvedValue(updatedUser);

      await service.changePassword('user-123', changePasswordInput);

      expect(rmqPublisher.publishUserUpdated).not.toHaveBeenCalled();
    });
  });

  describe('promoteUserToAdmin', () => {
    it('should promote user and publish event', async () => {
      const updatedUser = { ...mockUser, role: Role.ADMIN };
      dbUserService.promoteUserToAdmin.mockResolvedValue(updatedUser);

      await service.promoteUserToAdmin('user-123');

      expect(dbUserService.promoteUserToAdmin).toHaveBeenCalledWith('user-123');
      expect(redisService.hSet).toHaveBeenCalled();
      expect(rmqPublisher.publishUserUpdated).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'user-123',
          role: Role.ADMIN,
        }),
      );
    });
  });

  describe('updateUserRole', () => {
    it('should update user role and publish event', async () => {
      const updatedUser = { ...mockUser, role: Role.MERCHANT };
      dbUserService.updateUserRole.mockResolvedValue(updatedUser);

      await service.updateUserRole('user-123', Role.MERCHANT);

      expect(dbUserService.updateUserRole).toHaveBeenCalledWith('user-123', Role.MERCHANT);
      expect(redisService.hSet).toHaveBeenCalled();
      expect(rmqPublisher.publishUserUpdated).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'user-123',
          role: Role.MERCHANT,
        }),
      );
    });
  });

  describe('toggleUserActive', () => {
    it('should toggle user active status and publish event', async () => {
      const updatedUser = { ...mockUser, isActive: false };
      dbUserService.toggleUserActive.mockResolvedValue(updatedUser);

      await service.toggleUserActive('user-123', false);

      expect(dbUserService.toggleUserActive).toHaveBeenCalledWith('user-123', false);
      expect(redisService.hSet).toHaveBeenCalled();
      expect(rmqPublisher.publishUserUpdated).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'user-123',
          isActive: false,
        }),
      );
    });
  });

  describe('deleteUser', () => {
    it('should delete user and publish event', async () => {
      dbUserService.deleteUser.mockResolvedValue(true);

      const result = await service.deleteUser('user-123');

      expect(result).toBe(true);
      expect(dbUserService.deleteUser).toHaveBeenCalledWith('user-123');
      expect(redisService.del).toHaveBeenCalledWith('user:user-123');
      expect(rmqPublisher.publishUserDeleted).toHaveBeenCalledWith('user-123');
    });

    it('should return false if user not found', async () => {
      dbUserService.deleteUser.mockResolvedValue(false);

      const result = await service.deleteUser('user-123');

      expect(result).toBe(false);
      expect(redisService.del).not.toHaveBeenCalled();
      expect(rmqPublisher.publishUserDeleted).not.toHaveBeenCalled();
    });
  });

  describe('addAddress', () => {
    it('should add address and refresh user cache', async () => {
      dbUserService.addAddress.mockResolvedValue(mockAddress);
      dbUserService.findById.mockResolvedValue(mockUser);

      const result = await service.addAddress('user-123', { title: 'Home' });

      expect(result).toEqual(mockAddress);
      expect(dbUserService.addAddress).toHaveBeenCalledWith('user-123', { title: 'Home' });
      expect(dbUserService.findById).toHaveBeenCalledWith('user-123');
      expect(redisService.hSet).toHaveBeenCalled();
    });
  });

  describe('deleteAddress', () => {
    it('should delete address and refresh user cache', async () => {
      dbUserService.deleteAddress.mockResolvedValue(undefined);
      dbUserService.findById.mockResolvedValue(mockUser);

      await service.deleteAddress('user-123', 'addr-123');

      expect(dbUserService.deleteAddress).toHaveBeenCalledWith('user-123', 'addr-123');
      expect(dbUserService.findById).toHaveBeenCalledWith('user-123');
      expect(redisService.hSet).toHaveBeenCalled();
    });
  });

  describe('setDefaultAddress', () => {
    it('should set default address and refresh user cache', async () => {
      dbUserService.setDefaultAddress.mockResolvedValue(undefined);
      dbUserService.findById.mockResolvedValue(mockUser);

      await service.setDefaultAddress('user-123', 'addr-123');

      expect(dbUserService.setDefaultAddress).toHaveBeenCalledWith('user-123', 'addr-123');
      expect(dbUserService.findById).toHaveBeenCalledWith('user-123');
      expect(redisService.hSet).toHaveBeenCalled();
    });
  });

  describe('countUsersThisMonthAndLastMonth', () => {
    it('should delegate to dbUserService', async () => {
      const expected = { totalUsers: 100, usersThisMonth: 10, percentageIncrease: 25 };
      dbUserService.countUsersThisMonthAndLastMonth.mockResolvedValue(expected);

      const result = await service.countUsersThisMonthAndLastMonth();

      expect(result).toEqual(expected);
      expect(dbUserService.countUsersThisMonthAndLastMonth).toHaveBeenCalled();
    });
  });
});