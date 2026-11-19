import { HealthController } from './health.controller';
import { HealthService } from '@delivery-micro/shard';
import { DataSource } from 'typeorm';
import { RedisService } from '@bts-soft/core';

describe('HealthController', () => {
  let controller: HealthController;
  let healthService: jest.Mocked<HealthService>;
  let dataSource: jest.Mocked<DataSource>;
  let redisService: jest.Mocked<RedisService>;

  const systemStats = {
    uptime: 123.4,
    memory: {
      heapUsedMemoryMB: 42.5,
      rssMB: 128.7,
    },
    cpuUsage: { user: 1000, system: 500 },
  };

  /** Minimal express response double capturing status/json calls. */
  function mockResponse() {
    const res: any = {
      status: jest.fn(),
      json: jest.fn(),
    };
    res.status.mockReturnValue(res);
    return res;
  }

  beforeEach(() => {
    healthService = {
      getSystemStats: jest.fn().mockReturnValue(systemStats),
      checkDatabase: jest.fn(),
      checkRedis: jest.fn(),
    } as unknown as jest.Mocked<HealthService>;

    dataSource = {} as jest.Mocked<DataSource>;
    redisService = {} as jest.Mocked<RedisService>;

    controller = new HealthController(healthService, dataSource, redisService);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('GET /user/health (liveness)', () => {
    it('always answers UP with the service identity and system stats', () => {
      const result = controller.getHealth();

      expect(result.status).toBe('UP');
      expect(result.service).toBe('user-service');
      expect(Number.isNaN(Date.parse(result.timestamp))).toBe(false);
      expect(result.system).toBe(systemStats);
      expect(healthService.getSystemStats).toHaveBeenCalledTimes(1);
      // Liveness must not touch the dependencies at all.
      expect(healthService.checkDatabase).not.toHaveBeenCalled();
      expect(healthService.checkRedis).not.toHaveBeenCalled();
    });
  });

  describe('GET /user/ready (readiness)', () => {
    it('answers 200 UP when both the database and redis are reachable', async () => {
      healthService.checkDatabase.mockResolvedValue({ status: 'UP' } as any);
      healthService.checkRedis.mockResolvedValue({ status: 'UP' } as any);
      const res = mockResponse();

      await controller.getReadiness(res);

      expect(healthService.checkDatabase).toHaveBeenCalledWith(dataSource);
      expect(healthService.checkRedis).toHaveBeenCalledWith(redisService);
      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith(
        expect.objectContaining({
          status: 'UP',
          checks: {
            database: { status: 'UP' },
            redis: { status: 'UP' },
          },
          system: systemStats,
        }),
      );
      const payload = res.json.mock.calls[0][0];
      expect(Number.isNaN(Date.parse(payload.timestamp))).toBe(false);
    });

    it('answers 503 DOWN when the database check fails', async () => {
      healthService.checkDatabase.mockResolvedValue({
        status: 'DOWN',
        message: 'connection lost',
      } as any);
      healthService.checkRedis.mockResolvedValue({ status: 'UP' } as any);
      const res = mockResponse();

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(503);
      expect(res.json).toHaveBeenCalledWith(
        expect.objectContaining({
          status: 'DOWN',
          checks: {
            database: { status: 'DOWN', message: 'connection lost' },
            redis: { status: 'UP' },
          },
        }),
      );
    });

    it('answers 503 DOWN when redis is unavailable', async () => {
      healthService.checkDatabase.mockResolvedValue({ status: 'UP' } as any);
      healthService.checkRedis.mockResolvedValue({
        status: 'DOWN',
        message: 'NOAUTH',
      } as any);
      const res = mockResponse();

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(503);
      const payload = res.json.mock.calls[0][0];
      expect(payload.status).toBe('DOWN');
      expect(payload.checks.redis.status).toBe('DOWN');
    });

    it('answers 503 DOWN when both dependencies are unavailable', async () => {
      healthService.checkDatabase.mockResolvedValue({ status: 'DOWN' } as any);
      healthService.checkRedis.mockResolvedValue({ status: 'DOWN' } as any);
      const res = mockResponse();

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(503);
      expect(res.json.mock.calls[0][0].status).toBe('DOWN');
    });
  });
});
