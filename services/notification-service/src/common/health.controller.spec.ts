import { HealthController } from './health.controller';
import { HealthService } from '@delivery-micro/shard';
import { DataSource } from 'typeorm';
import { RedisService } from '@bts-soft/cache';

describe('HealthController', () => {
  let controller: HealthController;
  let healthService: {
    getSystemStats: jest.Mock;
    checkDatabase: jest.Mock;
    checkRedis: jest.Mock;
  };
  let dataSource: object;
  let redisService: object;

  const mockRes = () => {
    const res: any = {
      statusCode: 0,
      body: undefined,
      status: jest.fn(function (code: number) {
        res.statusCode = code;
        return res;
      }),
      json: jest.fn(function (payload: unknown) {
        res.body = payload;
        return res;
      }),
      setHeader: jest.fn(),
      end: jest.fn(),
    };
    return res;
  };

  beforeEach(() => {
    healthService = {
      getSystemStats: jest.fn().mockReturnValue({ uptime: 1 }),
      checkDatabase: jest.fn(),
      checkRedis: jest.fn(),
    };
    dataSource = {};
    redisService = {};

    controller = new HealthController(
      healthService as unknown as HealthService,
      dataSource as DataSource,
      redisService as RedisService,
    );
  });

  describe('getHealth', () => {
    it('returns UP with system stats', () => {
      const result = controller.getHealth();

      expect(result.status).toBe('UP');
      expect(result.service).toBe('notification-service');
      expect(result.system).toEqual({ uptime: 1 });
      expect(typeof result.timestamp).toBe('string');
    });
  });

  describe('getReadiness', () => {
    it('returns 200/UP when database and redis are healthy', async () => {
      healthService.checkDatabase.mockResolvedValue({ status: 'UP' });
      healthService.checkRedis.mockResolvedValue({ status: 'UP' });
      const res = mockRes();

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(200);
      expect(res.json).toHaveBeenCalledWith(
        expect.objectContaining({
          status: 'UP',
          checks: {
            database: { status: 'UP' },
            redis: { status: 'UP' },
          },
        }),
      );
    });

    it('returns 503/DOWN when the database is down', async () => {
      healthService.checkDatabase.mockResolvedValue({
        status: 'DOWN',
        message: 'connection refused',
      });
      healthService.checkRedis.mockResolvedValue({ status: 'UP' });
      const res = mockRes();

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(503);
      expect(res.json).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'DOWN' }),
      );
    });

    it('returns 503/DOWN when redis is down', async () => {
      healthService.checkDatabase.mockResolvedValue({ status: 'UP' });
      healthService.checkRedis.mockResolvedValue({ status: 'DOWN' });
      const res = mockRes();

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(503);
      expect(res.json).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'DOWN' }),
      );
    });
  });
});
