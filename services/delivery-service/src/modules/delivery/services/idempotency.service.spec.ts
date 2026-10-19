import { ConflictException } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { I18nService } from 'nestjs-i18n';
import { IdempotencyService } from './idempotency.service';
import { RedisService } from '@bts-soft/core';
import { createMockI18nService, createMockRedisService } from '../../../test-utils';

describe('IdempotencyService', () => {
  let service: IdempotencyService;
  let redisService: ReturnType<typeof createMockRedisService>;
  let i18nService: ReturnType<typeof createMockI18nService>;

  beforeEach(async () => {
    redisService = createMockRedisService();
    i18nService = createMockI18nService();
    const module: TestingModule = await Test.createTestingModule({
      providers: [
        IdempotencyService,
        { provide: RedisService, useValue: redisService },
        { provide: I18nService, useValue: i18nService },
      ],
    }).compile();

    service = module.get<IdempotencyService>(IdempotencyService);
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
  });

  describe('get', () => {
    it('should return cached value when key exists', async () => {
      const cachedValue = { id: 'delivery-123', status: 'CREATED' };
      redisService.get.mockResolvedValue(cachedValue);

      const result = await service.get('test-key');

      expect(result).toEqual(cachedValue);
      expect(redisService.get).toHaveBeenCalledWith('delivery:idempotency:test-key');
    });

    it('should return null when key does not exist', async () => {
      redisService.get.mockResolvedValue(null);

      const result = await service.get('non-existent-key');

      expect(result).toBeNull();
      expect(redisService.get).toHaveBeenCalledWith('delivery:idempotency:non-existent-key');
    });
  });

  describe('execute', () => {
    it('should return cached result when key exists', async () => {
      const cachedResult = { id: 'delivery-123', status: 'CREATED' };
      redisService.get.mockResolvedValue(cachedResult);

      const operation = jest.fn().mockResolvedValue({ id: 'new-delivery' });

      const result = await service.execute('test-key', operation);

      expect(result).toEqual(cachedResult);
      expect(operation).not.toHaveBeenCalled();
      expect(redisService.get).toHaveBeenCalledWith('delivery:idempotency:test-key');
      expect(redisService.setNX).not.toHaveBeenCalled();
    });

    it('should execute operation and cache result when key does not exist', async () => {
      redisService.get.mockResolvedValue(null);
      redisService.setNX.mockResolvedValue(true);
      redisService.set.mockResolvedValue('OK');

      const operationResult = { id: 'delivery-123', status: 'CREATED' };
      const operation = jest.fn().mockResolvedValue(operationResult);

      const result = await service.execute('test-key', operation);

      expect(result).toEqual(operationResult);
      expect(operation).toHaveBeenCalledTimes(1);
      expect(redisService.setNX).toHaveBeenCalledWith('delivery:idempotency:test-key:lock', '1', 30);
      expect(redisService.set).toHaveBeenCalledWith('delivery:idempotency:test-key', operationResult, 86400);
      expect(redisService.del).toHaveBeenCalledWith('delivery:idempotency:test-key:lock');
    });

    it('should throw ConflictException when lock cannot be acquired', async () => {
      redisService.get.mockResolvedValue(null);
      redisService.setNX.mockResolvedValue(false);

      const operation = jest.fn().mockResolvedValue({ id: 'delivery-123' });

      await expect(service.execute('test-key', operation)).rejects.toThrow(ConflictException);
      expect(operation).not.toHaveBeenCalled();
      expect(redisService.setNX).toHaveBeenCalledWith('delivery:idempotency:test-key:lock', '1', 30);
    });

    it('should use custom TTL when provided', async () => {
      redisService.get.mockResolvedValue(null);
      redisService.setNX.mockResolvedValue(true);
      redisService.set.mockResolvedValue('OK');

      const operationResult = { id: 'delivery-123' };
      const operation = jest.fn().mockResolvedValue(operationResult);
      const customTtl = 3600;

      await service.execute('test-key', operation, customTtl);

      expect(redisService.set).toHaveBeenCalledWith('delivery:idempotency:test-key', operationResult, customTtl);
    });

    it('should release lock even if operation throws', async () => {
      redisService.get.mockResolvedValue(null);
      redisService.setNX.mockResolvedValue(true);
      redisService.del.mockResolvedValue(1);

      const operation = jest.fn().mockRejectedValue(new Error('Operation failed'));

      await expect(service.execute('test-key', operation)).rejects.toThrow('Operation failed');
      expect(redisService.del).toHaveBeenCalledWith('delivery:idempotency:test-key:lock');
      expect(redisService.set).not.toHaveBeenCalled();
    });

    it('should not call operation if lock acquisition fails', async () => {
      redisService.get.mockResolvedValue(null);
      redisService.setNX.mockResolvedValue(false);

      const operation = jest.fn();

      await expect(service.execute('test-key', operation)).rejects.toThrow(ConflictException);
      expect(operation).not.toHaveBeenCalled();
    });
  });

  describe('clear', () => {
    it('should delete the idempotency key', async () => {
      redisService.del.mockResolvedValue(1);

      await service.clear('test-key');

      expect(redisService.del).toHaveBeenCalledWith('delivery:idempotency:test-key');
    });

    it('should not throw if key does not exist', async () => {
      redisService.del.mockResolvedValue(0);

      await expect(service.clear('non-existent-key')).resolves.not.toThrow();
    });
  });
});