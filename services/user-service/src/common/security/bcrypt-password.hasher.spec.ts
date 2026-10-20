import { Test, TestingModule } from '@nestjs/testing';
import { BcryptPasswordHasher } from './bcrypt-password.hasher';
import * as bcrypt from 'bcrypt';

jest.mock('bcrypt');

describe('BcryptPasswordHasher', () => {
  let service: BcryptPasswordHasher;
  let mockBcrypt: jest.Mocked<typeof bcrypt>;

  beforeEach(async () => {
    mockBcrypt = bcrypt as jest.Mocked<typeof bcrypt>;
    const module: TestingModule = await Test.createTestingModule({
      providers: [BcryptPasswordHasher],
    }).compile();

    service = module.get<BcryptPasswordHasher>(BcryptPasswordHasher);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('hash', () => {
    it('should hash plain text password', async () => {
      const plainText = 'password123';
      const hashedPassword = '$2b$10$hashedpassword12345678901234567890123456789012';
      mockBcrypt.hash.mockResolvedValue(hashedPassword);

      const result = await service.hash(plainText);

      expect(result).toBe(hashedPassword);
      expect(mockBcrypt.hash).toHaveBeenCalledWith(plainText, 10);
    });

    it('should use saltRounds of 10', async () => {
      mockBcrypt.hash.mockResolvedValue('hashed');

      await service.hash('password');

      expect(mockBcrypt.hash).toHaveBeenCalledWith('password', 10);
    });
  });

  describe('compare', () => {
    it('should return true for matching password and hash', async () => {
      const plainText = 'password123';
      const hash = '$2b$10$hashedpassword12345678901234567890123456789012';
      mockBcrypt.compare.mockResolvedValue(true);

      const result = await service.compare(plainText, hash);

      expect(result).toBe(true);
      expect(mockBcrypt.compare).toHaveBeenCalledWith(plainText, hash);
    });

    it('should return false for non-matching password and hash', async () => {
      const plainText = 'wrongpassword';
      const hash = '$2b$10$hashedpassword12345678901234567890123456789012';
      mockBcrypt.compare.mockResolvedValue(false);

      const result = await service.compare(plainText, hash);

      expect(result).toBe(false);
      expect(mockBcrypt.compare).toHaveBeenCalledWith(plainText, hash);
    });
  });
});