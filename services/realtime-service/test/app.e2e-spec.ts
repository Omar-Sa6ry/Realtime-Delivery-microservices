import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import request from 'supertest';
import { App } from 'supertest/types';
import { HealthController } from '../src/common/health.controller';
import { HealthService, MetricsService } from '@delivery-micro/shard';
import { RedisService } from '@bts-soft/cache';
import { RealtimeNatsService } from '../src/modules/infrastructure/nats/nats.service';
import { KafkaConsumer } from '../src/modules/infrastructure/kafka/kafka.consumer';
import { ConnectionService } from '../src/modules/gateway/connection/connection.service';

describe('HealthController (e2e)', () => {
  let app: INestApplication<App>;

  const healthService = {
    checkRedis: jest.fn(),
    getSystemStats: jest.fn(() => ({ uptime: 1 })),
  };
  const metricsService = {
    getMetrics: jest.fn(async () => '# HELP test'),
    getContentType: jest.fn(() => 'text/plain'),
  };
  const natsService = { isConnected: jest.fn(() => true) };
  const kafkaConsumer = { isConnected: jest.fn(() => true) };
  const connectionService = { getLocalSocketCount: jest.fn(() => 3) };

  beforeEach(async () => {
    jest.clearAllMocks();
    healthService.checkRedis.mockResolvedValue({ status: 'UP' });

    const moduleFixture: TestingModule = await Test.createTestingModule({
      controllers: [HealthController],
      providers: [
        { provide: HealthService, useValue: healthService },
        { provide: RedisService, useValue: {} },
        { provide: RealtimeNatsService, useValue: natsService },
        { provide: KafkaConsumer, useValue: kafkaConsumer },
        { provide: MetricsService, useValue: metricsService },
        { provide: ConnectionService, useValue: connectionService },
      ],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();
  });

  afterEach(async () => {
    await app.close();
  });

  it('/realtime/health (GET) returns UP when all dependencies are up', async () => {
    await request(app.getHttpServer())
      .get('/realtime/health')
      .expect(200)
      .expect((res) => {
        expect(res.body.status).toBe('UP');
        expect(res.body.service).toBe('realtime-service');
        expect(res.body.connections).toBe(3);
        expect(res.body.checks.redis.status).toBe('UP');
        expect(res.body.checks.nats.status).toBe('UP');
        expect(res.body.checks.kafka.status).toBe('UP');
      });
  });

  it('/realtime/health (GET) returns DEGRADED when kafka is down', async () => {
    kafkaConsumer.isConnected.mockReturnValue(false);

    await request(app.getHttpServer())
      .get('/realtime/health')
      .expect(200)
      .expect((res) => {
        expect(res.body.status).toBe('DEGRADED');
        expect(res.body.checks.kafka.status).toBe('DOWN');
      });
  });

  it('/realtime/health (GET) returns DOWN when redis is down', async () => {
    healthService.checkRedis.mockResolvedValue({ status: 'DOWN' });

    await request(app.getHttpServer())
      .get('/realtime/health')
      .expect(200)
      .expect((res) => {
        expect(res.body.status).toBe('DOWN');
      });
  });

  it('/realtime/ready (GET) returns 200 when redis and nats are up', async () => {
    await request(app.getHttpServer())
      .get('/realtime/ready')
      .expect(200)
      .expect((res) => {
        expect(res.body.status).toBe('UP');
      });
  });

  it('/realtime/ready (GET) returns 503 when nats is down', async () => {
    natsService.isConnected.mockReturnValue(false);

    await request(app.getHttpServer())
      .get('/realtime/ready')
      .expect(503)
      .expect((res) => {
        expect(res.body.status).toBe('DOWN');
      });
  });

  it('/realtime/metrics (GET) returns prometheus text', async () => {
    await request(app.getHttpServer())
      .get('/realtime/metrics')
      .expect(200)
      .expect('Content-Type', /text\/plain/)
      .expect((res) => {
        expect(res.text).toContain('# HELP test');
      });
  });
});
