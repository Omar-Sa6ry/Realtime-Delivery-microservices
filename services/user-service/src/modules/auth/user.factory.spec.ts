import { Test, TestingModule } from '@nestjs/testing';
import { UserFactory } from './user.factory';
import { User } from '../../common/database/entities/user.entity';
import { Role } from '@delivery-micro/shard';
import { IdGenerator } from '@bts-soft/core';

jest.mock('@bts-soft/core', () => ({
  IdGenerator: {
    generate: jest.fn().mockReturnValue('generated-id-123'),
  },
}));

describe('UserFactory', () => {
  let factory: UserFactory;

  beforeEach(async () => {
    const module: TestingModule = await Test.createTestingModule({
      providers: [UserFactory],
    }).compile();

    factory = module.get<UserFactory>(UserFactory);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('createUser', () => {
    it('should create user with all required fields', () => {
      const user = factory.createUser(
        'test@example.com',
        'hashed-password',
        'John',
        'Doe',
        Role.USER,
        '+1234567890',
        'https://example.com/avatar.jpg',
      );

      expect(user).toBeInstanceOf(User);
      expect(user.id).toBe('generated-id-123');
      expect(user.email).toBe('test@example.com');
      expect(user.passwordHash).toBe('hashed-password');
      expect(user.firstName).toBe('John');
      expect(user.lastName).toBe('Doe');
      expect(user.phoneNumber).toBe('+1234567890');
      expect(user.role).toBe(Role.USER);
      expect(user.isActive).toBe(true);
      expect(user.imageUrl).toBe('https://example.com/avatar.jpg');
      expect(user.addresses).toEqual([]);
    });

    it('should lowercase and trim email', () => {
      const user = factory.createUser(
        '  TEST@EXAMPLE.COM  ',
        'hashed',
        'John',
        'Doe',
        Role.USER,
      );

      expect(user.email).toBe('test@example.com');
    });

    it('should trim phone number', () => {
      const user = factory.createUser(
        'test@example.com',
        'hashed',
        'John',
        'Doe',
        Role.USER,
        '  +1234567890  ',
      );

      expect(user.phoneNumber).toBe('+1234567890');
    });

    it('should set empty string for undefined phone number', () => {
      const user = factory.createUser(
        'test@example.com',
        'hashed',
        'John',
        'Doe',
        Role.USER,
        undefined,
      );

      expect(user.phoneNumber).toBe('');
    });

    it('should set imageUrl to undefined if not provided', () => {
      const user = factory.createUser(
        'test@example.com',
        'hashed',
        'John',
        'Doe',
        Role.USER,
      );

      expect(user.imageUrl).toBeUndefined();
    });

    it('should generate unique ID using IdGenerator', () => {
      factory.createUser('test@example.com', 'hashed', 'John', 'Doe', Role.USER);

      expect(IdGenerator.generate).toHaveBeenCalledWith('snowflake');
    });

    it('should create user with ADMIN role', () => {
      const user = factory.createUser(
        'admin@example.com',
        'hashed',
        'Admin',
        'User',
        Role.ADMIN,
      );

      expect(user.role).toBe(Role.ADMIN);
    });

    it('should create user with MERCHANT role', () => {
      const user = factory.createUser(
        'merchant@example.com',
        'hashed',
        'Merchant',
        'User',
        Role.MERCHANT,
      );

      expect(user.role).toBe(Role.MERCHANT);
    });
  });
});