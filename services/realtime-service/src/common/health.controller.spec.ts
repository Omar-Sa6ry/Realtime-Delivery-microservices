import { HealthController } from './health.controller';
import { HealthService, MetricsService } from '@delivery-micro/shard';
import { RedisService } from '@bts-soft/cache';
import { RealtimeNatsService } from '../../modules/infrastructure/nats/nats.service';
import { KafkaConsumer } from '../../modules/infrastructure/kafka/kafka.consumer';
import { ConnectionService } from '../../modules/gateway/connection/connection.service';

describe('HealthController', () => {
  let controller: HealthController;
  let healthService: jest.Mocked<HealthService>;
  let metricsService: jest.Mocked<MetricsService>;
  let natsService: { isConnected: jest.Mock };
  let kafkaConsumer: { isConnected: jest.Mock };
  let connectionService: { getLocalSocketCount: jest.Mock };
  let res: any;

  beforeEach(() => {
    healthService = {
      checkRedis: jest.fn(),
      getSystemStats: jest.fn(() => ({ uptime: 10 })),
    } as any;
    metricsService = {
      getMetrics: jest.fn(async () => 'metrics-text'),
      getContentType: jest.fn(() => 'text/plain; version=0.0.4'),
    } as any;
    natsService = { isConnected: jest.fn(() => true) };
    kafkaConsumer = { isConnected: jest.fn(() => true) };
    connectionService = { getLocalSocketCount: jest.fn(() => 7) };

    controller = new HealthController(
      healthService,
      {} as RedisService,
      natsService as unknown as RealtimeNatsService,
      kafkaConsumer as unknown as KafkaConsumer,
      metricsService,
      connectionService as unknown as ConnectionService,
    );

    res = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn().mockReturnThis(),
      setHeader: jest.fn(),
      send: jest.fn(),
    };
  });

  describe('getHealth', () => {
    it('returns UP when redis, nats and kafka are up', async () => {
      healthService.checkRedis.mockResolvedValue({ status: 'UP' });

      await controller.getHealth(res);

      expect(res.status).toHaveBeenCalledWith(200);
      const body = res.json.mock.calls[0][0];
      expect(body.status).toBe('UP');
      expect(body.service).toBe('realtime-service');
      expect(body.connections).toBe(7);
      expect(body.checks.redis.status).toBe('UP');
      expect(body.checks.nats.status).toBe('UP');
      expect(body.checks.kafka.status).toBe('UP');
      expect(body.system).toEqual({ uptime: 10 });
    });

    it('returns DEGRADED when kafka is down but critical deps are up', async () => {
      healthService.checkRedis.mockResolvedValue({ status: 'UP' });
      kafkaConsumer.isConnected.mockReturnValue(false);

      await controller.getHealth(res);

      const body = res.json.mock.calls[0][0];
      expect(body.status).toBe('DEGRADED');
      expect(body.checks.kafka.status).toBe('DOWN');
      expect(body.checks.kafka.message).toBe('Kafka consumer not running');
    });

    it('returns DOWN when redis is down', async () => {
      healthService.checkRedis.mockResolvedValue({ status: 'DOWN' });
      natsService.isConnected.mockReturnValue(false);

      await controller.getHealth(res);

      const body = res.json.mock.calls[0][0];
      expect(body.status).toBe('DOWN');
    });

    it('reports nats down message when not connected', async () => {
      healthService.checkRedis.mockResolvedValue({ status: 'UP' });
      natsService.isConnected.mockReturnValue(false);

      await controller.getHealth(res);

      const body = res.json.mock.calls[0][0];
      expect(body.checks.nats.message).toBe('NATS not connected');
    });
  });

  describe('getReadiness', () => {
    it('returns 200 when redis and nats are up', async () => {
      healthService.checkRedis.mockResolvedValue({ status: 'UP' });

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(200);
      const body = res.json.mock.calls[0][0];
      expect(body.status).toBe('UP');
    });

    it('returns 503 when redis is down', async () => {
      healthService.checkRedis.mockResolvedValue({ status: 'DOWN' });

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(503);
      expect(res.json.mock.calls[0][0].status).toBe('DOWN');
    });

    it('returns 503 when nats is down', async () => {
      healthService.checkRedis.mockResolvedValue({ status: 'UP' });
      natsService.isConnected.mockReturnValue(false);

      await controller.getReadiness(res);

      expect(res.status).toHaveBeenCalledWith(503);
    });
  });

  describe('getMetrics', () => {
    it('serves prometheus metrics with correct content type', async () => {
      await controller.getMetrics(res);

      expect(res.setHeader).toHaveBeenCalledWith(
        'Content-Type',
        'text/plain; version=0.0.4',
      );
      expect(res.send).toHaveBeenCalledWith('metrics-text');
    });
  });
});
