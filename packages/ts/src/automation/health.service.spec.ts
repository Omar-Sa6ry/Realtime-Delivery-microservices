import { Logger } from '@nestjs/common';
import { HealthService } from './health.service';

describe('HealthService', () => {
  let service: HealthService;
  let errorSpy: jest.SpyInstance;

  beforeEach(() => {
    service = new HealthService();
    errorSpy = jest.spyOn(Logger.prototype, 'error').mockImplementation();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('checkDatabase', () => {
    it('reports UP when the ping query resolves', async () => {
      const dataSource = { query: jest.fn().mockResolvedValue([{ '1': 1 }]) };

      const result = await service.checkDatabase(dataSource);

      expect(dataSource.query).toHaveBeenCalledWith('SELECT 1');
      expect(result).toEqual({ status: 'UP' });
      expect(errorSpy).not.toHaveBeenCalled();
    });

    it('reports DOWN with the failure message when the query rejects', async () => {
      const error = new Error('connection lost');
      const dataSource = { query: jest.fn().mockRejectedValue(error) };

      const result = await service.checkDatabase(dataSource);

      expect(result).toEqual({ status: 'DOWN', message: 'connection lost' });
      expect(errorSpy).toHaveBeenCalledWith(
        'Health Check: Database connection failed',
        error.stack,
      );
    });
  });

  describe('checkRedis', () => {
    it('uses a direct ping method when available', async () => {
      const redis = { ping: jest.fn().mockResolvedValue('PONG') };

      const result = await service.checkRedis(redis);

      expect(redis.ping).toHaveBeenCalledTimes(1);
      expect(result).toEqual({ status: 'UP' });
    });

    it('falls back to the nested client ping method', async () => {
      const redis = { client: { ping: jest.fn().mockResolvedValue('PONG') } };

      const result = await service.checkRedis(redis);

      expect(redis.client.ping).toHaveBeenCalledTimes(1);
      expect(result).toEqual({ status: 'UP' });
    });

    it('reports UP when no ping method exists at all', async () => {
      const result = await service.checkRedis({});

      expect(result).toEqual({ status: 'UP' });
    });

    it('reports DOWN when ping rejects', async () => {
      const error = new Error('redis timeout');
      const redis = { ping: jest.fn().mockRejectedValue(error) };

      const result = await service.checkRedis(redis);

      expect(result).toEqual({ status: 'DOWN', message: 'redis timeout' });
      expect(errorSpy).toHaveBeenCalledWith(
        'Health Check: Redis connection failed',
        error.stack,
      );
    });
  });

  describe('checkRabbitMQ', () => {
    it('reports UNKNOWN when no service was injected', async () => {
      const result = await service.checkRabbitMQ(null);

      expect(result).toEqual({
        status: 'UNKNOWN',
        message: 'RabbitMQ service not injected',
      });
      expect(errorSpy).not.toHaveBeenCalled();
    });

    it('reports UP when connected', async () => {
      const rabbit = { isConnected: jest.fn().mockReturnValue(true) };

      const result = await service.checkRabbitMQ(rabbit);

      expect(rabbit.isConnected).toHaveBeenCalledTimes(1);
      expect(result).toEqual({ status: 'UP' });
    });

    it('reports DOWN when not connected', async () => {
      const rabbit = { isConnected: jest.fn().mockReturnValue(false) };

      const result = await service.checkRabbitMQ(rabbit);

      expect(result).toEqual({
        status: 'DOWN',
        message: 'RabbitMQ not connected',
      });
    });

    it('assumes connected when no isConnected method exists', async () => {
      const result = await service.checkRabbitMQ({});

      expect(result).toEqual({ status: 'UP' });
    });

    it('reports DOWN when the connection check throws', async () => {
      const error = new Error('broker unreachable');
      const rabbit = {
        isConnected: jest.fn(() => {
          throw error;
        }),
      };

      const result = await service.checkRabbitMQ(rabbit);

      expect(result).toEqual({ status: 'DOWN', message: 'broker unreachable' });
      expect(errorSpy).toHaveBeenCalledWith(
        'Health Check: RabbitMQ connection failed',
        error.stack,
      );
    });
  });

  describe('getSystemStats', () => {
    it('returns uptime, rounded memory and cpu usage', () => {
      const memorySpy = jest.spyOn(process, 'memoryUsage').mockReturnValue({
        heapTotal: 3 * 1024 * 1024,
        heapUsed: 2 * 1024 * 1024,
        rss: 5 * 1024 * 1024,
      } as NodeJS.MemoryUsage);
      const uptimeSpy = jest.spyOn(process, 'uptime').mockReturnValue(123.4);
      const cpuSpy = jest
        .spyOn(process, 'cpuUsage')
        .mockReturnValue({ user: 11, system: 22 });

      const stats = service.getSystemStats();

      expect(memorySpy).toHaveBeenCalledTimes(1);
      expect(uptimeSpy).toHaveBeenCalledTimes(1);
      expect(cpuSpy).toHaveBeenCalledTimes(1);
      expect(stats).toEqual({
        uptime: 123.4,
        memory: {
          heapTotalMemoryMB: 3,
          heapUsedMemoryMB: 2,
          rssMB: 5,
        },
        cpuUsage: { user: 11, system: 22 },
      });
    });

    it('returns real process statistics by default', () => {
      const stats = service.getSystemStats();

      expect(typeof stats.uptime).toBe('number');
      expect(stats.uptime).toBeGreaterThan(0);
      expect(typeof stats.memory.heapTotalMemoryMB).toBe('number');
      expect(typeof stats.memory.heapUsedMemoryMB).toBe('number');
      expect(typeof stats.memory.rssMB).toBe('number');
      expect(typeof stats.cpuUsage.user).toBe('number');
      expect(typeof stats.cpuUsage.system).toBe('number');
    });
  });
});
