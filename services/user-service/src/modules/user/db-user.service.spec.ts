import { Test, TestingModule } from '@nestjs/testing';
import { BadRequestException, UnauthorizedException, NotFoundException } from '@nestjs/common';
import { DbUserService } from './db-user.service';
import { Repository, DataSource, QueryRunner, EntityManager } from 'typeorm';
import { User } from '../../common/database/entities/user.entity';
import { Address } from '../../common/database/entities/address.entity';
import { Outbox } from '../../common/database/entities/outbox.entity';
import { BcryptPasswordHasher } from '../../common/security/bcrypt-password.hasher';
import { OutboxWorkerService } from '../../common/messaging/outbox-worker.service';
import { MediaGrpcService } from '../media/grpc-media.service';
import { KafkaService } from '@delivery-micro/shard';
import { I18nService } from 'nestjs-i18n';
import { UpdateProfileInput, ChangePasswordInput } from './dto/user.types';
import { Role, UserKafkaTopics } from '@delivery-micro/shard';

describe('DbUserService', () => {
  let service: DbUserService;
  let userRepo: jest.Mocked<Repository<User>>;
  let addressRepo: jest.Mocked<Repository<Address>>;
  let passwordHasher: jest.Mocked<BcryptPasswordHasher>;
  let dataSource: jest.Mocked<DataSource>;
  let queryRunner: jest.Mocked<QueryRunner>;
  let outboxWorkerService: jest.Mocked<OutboxWorkerService>;
  let mediaGrpcService: jest.Mocked<MediaGrpcService>;
  let kafkaService: jest.Mocked<KafkaService>;
  let i18n: jest.Mocked<I18nService>;

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
    userRepo = {
      findOne: jest.fn(),
      save: jest.fn(),
      delete: jest.fn(),
      count: jest.fn(),
    } as unknown as jest.Mocked<Repository<User>>;

    addressRepo = {
      save: jest.fn(),
      delete: jest.fn(),
    } as unknown as jest.Mocked<Repository<Address>>;

    passwordHasher = {
      hash: jest.fn(),
      compare: jest.fn(),
    } as unknown as jest.Mocked<BcryptPasswordHasher>;

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

    mediaGrpcService = {
      resolveMediaUrl: jest.fn(),
    } as unknown as jest.Mocked<MediaGrpcService>;

    kafkaService = {
      emit: jest.fn(),
    } as unknown as jest.Mocked<KafkaService>;

    i18n = {
      t: jest.fn((key: string) => key),
    } as unknown as jest.Mocked<I18nService>;

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DbUserService,
        { provide: 'UserRepository', useValue: userRepo },
        { provide: 'AddressRepository', useValue: addressRepo },
        { provide: BcryptPasswordHasher, useValue: passwordHasher },
        { provide: DataSource, useValue: dataSource },
        { provide: OutboxWorkerService, useValue: outboxWorkerService },
        { provide: MediaGrpcService, useValue: mediaGrpcService },
        { provide: KafkaService, useValue: kafkaService },
        { provide: I18nService, useValue: i18n },
      ],
    }).compile();

    service = module.get<DbUserService>(DbUserService);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('findById', () => {
    it('should return user if found', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);

      const result = await service.findById('user-123');

      expect(result).toEqual(mockUser);
      expect(userRepo.findOne).toHaveBeenCalledWith({ where: { id: 'user-123' } });
    });

    it('should return null if not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      const result = await service.findById('user-123');

      expect(result).toBeNull();
    });
  });

  describe('findByEmail', () => {
    it('should return user if found', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);

      const result = await service.findByEmail('test@example.com');

      expect(result).toEqual(mockUser);
      expect(userRepo.findOne).toHaveBeenCalledWith({
        where: { email: 'test@example.com' },
      });
    });

    it('should trim and lowercase email', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);

      await service.findByEmail('  TEST@EXAMPLE.COM  ');

      expect(userRepo.findOne).toHaveBeenCalledWith({
        where: { email: 'test@example.com' },
      });
    });
  });

  describe('updateProfile', () => {
    const updateInput: UpdateProfileInput = {
      firstName: 'Jane',
      lastName: 'Smith',
      imageUrl: 'https://example.com/new-avatar.jpg',
      avatarMediaId: undefined,
    };

    beforeEach(() => {
      userRepo.findOne.mockResolvedValue(mockUser);
      queryRunner.manager.save.mockResolvedValue({ ...mockUser, firstName: 'Jane', lastName: 'Smith' });
    });

    it('should update profile and create outbox event', async () => {
      const result = await service.updateProfile('user-123', updateInput);

      expect(result).toBeDefined();
      expect(userRepo.findOne).toHaveBeenCalledWith({ where: { id: 'user-123' } });
      expect(queryRunner.connect).toHaveBeenCalled();
      expect(queryRunner.startTransaction).toHaveBeenCalled();
      expect(queryRunner.commitTransaction).toHaveBeenCalled();
      expect(queryRunner.release).toHaveBeenCalled();
      expect(outboxWorkerService.enqueueEvent).toHaveBeenCalled();
      expect(kafkaService.emit).toHaveBeenCalledWith(
        UserKafkaTopics.USER_UPDATED,
        UserKafkaTopics.USER_UPDATED,
        expect.objectContaining({
          userId: 'user-123',
          email: 'test@example.com',
          firstName: 'Jane',
          lastName: 'Smith',
        }),
      );
    });

    it('should throw BadRequestException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.updateProfile('user-123', updateInput)).rejects.toThrow(
        BadRequestException,
      );
    });

    it('should rollback transaction on error', async () => {
      queryRunner.manager.save.mockRejectedValue(new Error('DB error'));

      await expect(service.updateProfile('user-123', updateInput)).rejects.toThrow('DB error');
      expect(queryRunner.rollbackTransaction).toHaveBeenCalled();
    });

    it('should resolve avatar media URL if avatarMediaId provided', async () => {
      const inputWithMedia: UpdateProfileInput = {
        ...updateInput,
        avatarMediaId: 'media-123',
      };
      mediaGrpcService.resolveMediaUrl.mockResolvedValue({
        url: 'https://media.example.com/avatar.jpg',
        expiresAtSeconds: 3600,
        contentType: 'image/jpeg',
        mediaId: 'media-123',
        status: 'ready',
      });

      await service.updateProfile('user-123', inputWithMedia);

      expect(mediaGrpcService.resolveMediaUrl).toHaveBeenCalledWith(
        expect.objectContaining({
          mediaId: 'media-123',
          requesterId: 'user-123',
        }),
      );
    });

    it('should retry media URL resolution once on failure', async () => {
      const inputWithMedia: UpdateProfileInput = {
        ...updateInput,
        avatarMediaId: 'media-123',
      };
      mediaGrpcService.resolveMediaUrl
        .mockRejectedValueOnce(new Error('Temporary error'))
        .mockResolvedValueOnce({
          url: 'https://media.example.com/avatar.jpg',
          expiresAtSeconds: 3600,
          contentType: 'image/jpeg',
          mediaId: 'media-123',
          status: 'ready',
        });

      await service.updateProfile('user-123', inputWithMedia);

      expect(mediaGrpcService.resolveMediaUrl).toHaveBeenCalledTimes(2);
    });

    it('should set imageUrl to undefined if both retries fail', async () => {
      const inputWithMedia: UpdateProfileInput = {
        ...updateInput,
        avatarMediaId: 'media-123',
      };
      mediaGrpcService.resolveMediaUrl.mockRejectedValue(new Error('Persistent error'));
      userRepo.findOne.mockResolvedValue(mockUser);
      queryRunner.manager.save.mockResolvedValue({ ...mockUser, imageUrl: undefined });

      const result = await service.updateProfile('user-123', inputWithMedia);

      expect(result.imageUrl).toBeUndefined();
    });

    it('should validate imageUrl if provided without avatarMediaId', async () => {
      const invalidInput: UpdateProfileInput = {
        ...updateInput,
        imageUrl: 'not-a-valid-url',
      };

      await expect(service.updateProfile('user-123', invalidInput)).rejects.toThrow(
        BadRequestException,
      );
    });
  });

  describe('changePassword', () => {
    const changePasswordInput: ChangePasswordInput = {
      passwordOld: 'oldPassword123',
      passwordNew: 'newPassword456',
    };

    it('should change password if old password is valid', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      passwordHasher.compare.mockResolvedValue(true);
      passwordHasher.hash.mockResolvedValue('new-hashed-password');
      userRepo.save.mockResolvedValue({ ...mockUser, passwordHash: 'new-hashed-password' });

      const result = await service.changePassword('user-123', changePasswordInput);

      expect(result).toBeDefined();
      expect(passwordHasher.compare).toHaveBeenCalledWith(
        'oldPassword123',
        'hashed-password',
      );
      expect(passwordHasher.hash).toHaveBeenCalledWith('newPassword456');
      expect(userRepo.save).toHaveBeenCalled();
    });

    it('should throw BadRequestException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.changePassword('user-123', changePasswordInput)).rejects.toThrow(
        BadRequestException,
      );
    });

    it('should throw UnauthorizedException if old password is invalid', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      passwordHasher.compare.mockResolvedValue(false);

      await expect(service.changePassword('user-123', changePasswordInput)).rejects.toThrow(
        UnauthorizedException,
      );
    });
  });

  describe('promoteUserToAdmin', () => {
    it('should promote user to admin', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      userRepo.save.mockResolvedValue({ ...mockUser, role: Role.ADMIN });

      const result = await service.promoteUserToAdmin('user-123');

      expect(result.role).toBe(Role.ADMIN);
      expect(userRepo.save).toHaveBeenCalled();
      expect(kafkaService.emit).toHaveBeenCalled();
    });

    it('should throw NotFoundException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.promoteUserToAdmin('user-123')).rejects.toThrow(NotFoundException);
    });
  });

  describe('updateUserRole', () => {
    it('should update user role', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      userRepo.save.mockResolvedValue({ ...mockUser, role: Role.MERCHANT });

      const result = await service.updateUserRole('user-123', 'merchant');

      expect(result.role).toBe(Role.MERCHANT);
      expect(userRepo.save).toHaveBeenCalled();
    });

    it('should throw NotFoundException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.updateUserRole('user-123', 'merchant')).rejects.toThrow(
        NotFoundException,
      );
    });
  });

  describe('toggleUserActive', () => {
    it('should toggle user active status', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      userRepo.save.mockResolvedValue({ ...mockUser, isActive: false });

      const result = await service.toggleUserActive('user-123', false);

      expect(result.isActive).toBe(false);
      expect(userRepo.save).toHaveBeenCalled();
    });

    it('should throw NotFoundException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.toggleUserActive('user-123', false)).rejects.toThrow(
        NotFoundException,
      );
    });
  });

  describe('deleteUser', () => {
    it('should delete user and publish event', async () => {
      userRepo.findOne.mockResolvedValue(mockUser);
      userRepo.delete.mockResolvedValue({ affected: 1, raw: [] });

      const result = await service.deleteUser('user-123');

      expect(result).toBe(true);
      expect(userRepo.delete).toHaveBeenCalledWith('user-123');
      expect(kafkaService.emit).toHaveBeenCalledWith(
        UserKafkaTopics.USER_DELETED,
        UserKafkaTopics.USER_DELETED,
        expect.objectContaining({
          userId: 'user-123',
        }),
      );
    });

    it('should throw NotFoundException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.deleteUser('user-123')).rejects.toThrow(NotFoundException);
    });
  });

  describe('addAddress', () => {
    it('should add address to user', async () => {
      userRepo.findOne.mockResolvedValue({ ...mockUser, addresses: [] });
      addressRepo.save.mockResolvedValue(mockAddress);

      const result = await service.addAddress('user-123', {
        title: 'Home',
        street: '123 Main St',
        city: 'New York',
      });

      expect(result).toEqual(mockAddress);
      expect(addressRepo.save).toHaveBeenCalled();
    });

    it('should throw NotFoundException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.addAddress('user-123', { title: 'Home' })).rejects.toThrow(
        NotFoundException,
      );
    });

    it('should set other addresses to non-default when new address is default', async () => {
      const existingAddress = { ...mockAddress, isDefault: true };
      userRepo.findOne.mockResolvedValue({ ...mockUser, addresses: [existingAddress] });
      addressRepo.save.mockResolvedValue({ ...mockAddress, isDefault: true });

      await service.addAddress('user-123', {
        title: 'Work',
        street: '456 Work St',
        city: 'New York',
        isDefault: true,
      });

      expect(existingAddress.isDefault).toBe(false);
      expect(addressRepo.save).toHaveBeenCalledTimes(2);
    });
  });

  describe('deleteAddress', () => {
    it('should delete address if belongs to user', async () => {
      userRepo.findOne.mockResolvedValue({ ...mockUser, addresses: [mockAddress] });
      addressRepo.delete.mockResolvedValue({ affected: 1, raw: [] });

      await service.deleteAddress('user-123', 'addr-123');

      expect(addressRepo.delete).toHaveBeenCalledWith('addr-123');
    });

    it('should throw NotFoundException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.deleteAddress('user-123', 'addr-123')).rejects.toThrow(
        NotFoundException,
      );
    });

    it('should throw BadRequestException if address not found for user', async () => {
      userRepo.findOne.mockResolvedValue({ ...mockUser, addresses: [] });

      await expect(service.deleteAddress('user-123', 'addr-123')).rejects.toThrow(
        BadRequestException,
      );
    });
  });

  describe('setDefaultAddress', () => {
    it('should set address as default', async () => {
      const addr1 = { ...mockAddress, id: 'addr-1', isDefault: true };
      const addr2 = { ...mockAddress, id: 'addr-2', isDefault: false };
      userRepo.findOne.mockResolvedValue({ ...mockUser, addresses: [addr1, addr2] });
      addressRepo.save.mockResolvedValue([{ ...addr1, isDefault: false }, { ...addr2, isDefault: true }]);

      await service.setDefaultAddress('user-123', 'addr-2');

      expect(addr1.isDefault).toBe(false);
      expect(addr2.isDefault).toBe(true);
      expect(addressRepo.save).toHaveBeenCalledWith([addr1, addr2]);
    });

    it('should throw NotFoundException if user not found', async () => {
      userRepo.findOne.mockResolvedValue(null);

      await expect(service.setDefaultAddress('user-123', 'addr-123')).rejects.toThrow(
        NotFoundException,
      );
    });

    it('should throw BadRequestException if address not found for user', async () => {
      userRepo.findOne.mockResolvedValue({ ...mockUser, addresses: [] });

      await expect(service.setDefaultAddress('user-123', 'addr-123')).rejects.toThrow(
        BadRequestException,
      );
    });
  });

  describe('countUsersThisMonthAndLastMonth', () => {
    it('should return user counts and percentage increase', async () => {
      userRepo.count
        .mockResolvedValueOnce(100) // totalUsers
        .mockResolvedValueOnce(10) // usersThisMonth
        .mockResolvedValueOnce(8); // usersLastMonth

      const result = await service.countUsersThisMonthAndLastMonth();

      expect(result).toEqual({
        totalUsers: 100,
        usersThisMonth: 10,
        percentageIncrease: 25,
      });
    });

    it('should return 100% increase if no users last month', async () => {
      userRepo.count
        .mockResolvedValueOnce(100)
        .mockResolvedValueOnce(10)
        .mockResolvedValueOnce(0);

      const result = await service.countUsersThisMonthAndLastMonth();

      expect(result.percentageIncrease).toBe(100);
    });
  });
});