import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import { DataSource } from 'typeorm';
import request from 'supertest';
import { App } from 'supertest/types';
import { HealthService, MetricsService } from '@delivery-micro/shard';
import { RedisService } from '@bts-soft/cache';
import { HealthController } from '../src/common/health.controller';
import { MetricsController } from '../src/common/metrics.controller';

describe('Health & Metrics endpoints (e2e)', () => {
  let app: INestApplication<App>;
  let healthService: {
    getSystemStats: jest.Mock;
    checkDatabase: jest.Mock;
    checkRedis: jest.Mock;
  };
  let metricsService: {
    getMetrics: jest.Mock;
    getContentType: jest.Mock;
  };

  beforeEach(async () => {
    healthService = {
      getSystemStats: jest.fn().mockReturnValue({
        uptime: 12,
        memory: { heapTotalMemoryMB: 1, heapUsedMemoryMB: 1, rssMB: 2 },
        cpuUsage: { user: 0, system: 0 },
      }),
      checkDatabase: jest.fn().mockResolvedValue({ status: 'UP' }),
      checkRedis: jest.fn().mockResolvedValue({ status: 'UP' }),
    };
    metricsService = {
      getMetrics: jest
        .fn()
        .mockResolvedValue('# HELP notification_requests_total 1\n'),
      getContentType: jest
        .fn()
        .mockReturnValue('text/plain; version=0.0.4; charset=utf-8'),
    };

    const moduleFixture: TestingModule = await Test.createTestingModule({
      controllers: [HealthController, MetricsController],
      providers: [
        { provide: HealthService, useValue: healthService },
        { provide: MetricsService, useValue: metricsService },
        { provide: DataSource, useValue: {} },
        { provide: RedisService, useValue: {} },
      ],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();
  });

  afterEach(async () => {
    await app.close();
  });

  describe('GET /notification/health', () => {
    it('returns UP with service metadata', async () => {
      const res = await request(app.getHttpServer())
        .get('/notification/health')
        .expect(200);

      expect(res.body).toMatchObject({
        status: 'UP',
        service: 'notification-service',
        system: { uptime: 12 },
      });
      expect(typeof res.body.timestamp).toBe('string');
    });
  });

  describe('GET /notification/ready', () => {
    it('returns 200 UP when database and redis are healthy', async () => {
      const res = await request(app.getHttpServer())
        .get('/notification/ready')
        .expect(200);

      expect(res.body.status).toBe('UP');
      expect(res.body.checks.database.status).toBe('UP');
      expect(res.body.checks.redis.status).toBe('UP');
      expect(healthService.checkDatabase).toHaveBeenCalled();
      expect(healthService.checkRedis).toHaveBeenCalled();
    });

    it('returns 503 DOWN when a dependency is unhealthy', async () => {
      healthService.checkRedis.mockResolvedValue({
        status: 'DOWN',
        message: 'redis unreachable',
      });

      const res = await request(app.getHttpServer())
        .get('/notification/ready')
        .expect(503);

      expect(res.body.status).toBe('DOWN');
      expect(res.body.checks.redis.status).toBe('DOWN');
    });
  });

  describe('GET /notification/metrics', () => {
    it('serves prometheus metrics with the proper content type', async () => {
      const res = await request(app.getHttpServer())
        .get('/notification/metrics')
        .expect(200);

      expect(res.headers['content-type']).toContain('text/plain');
      expect(res.text).toContain('notification_requests_total');
    });
  });
});
