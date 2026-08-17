import { Controller, Get, Res } from '@nestjs/common';
import type { Response } from 'express';
import { HealthService, MetricsService } from '@delivery/common';
import { RedisService } from '@bts-soft/cache';
import { RealtimeNatsService } from '../modules/nats/nats.service';
import { KafkaConsumer } from '../modules/kafka/kafka.consumer';
import { ConnectionService } from '../modules/connection/connection.service';

@Controller('realtime')
export class HealthController {
  constructor(
    private readonly healthService: HealthService,
    private readonly redisService: RedisService,
    private readonly natsService: RealtimeNatsService,
    private readonly kafkaConsumer: KafkaConsumer,
    private readonly metricsService: MetricsService,
    private readonly connectionService: ConnectionService,
  ) {}

  @Get('health')
  async getHealth(@Res() res: Response) {
    const [redisHealth, natsHealth, kafkaHealth] = await Promise.all([
      this.healthService.checkRedis(this.redisService),
      this.checkNats(),
      this.checkKafka(),
    ]);
    const system = this.healthService.getSystemStats();
    const connections = this.connectionService.getLocalSocketCount();

    const criticalUp = redisHealth.status === 'UP' && natsHealth.status === 'UP';
    const kafkaUp = kafkaHealth.status === 'UP';
    const status = criticalUp && kafkaUp ? 'UP' : criticalUp ? 'DEGRADED' : 'DOWN';

    res.status(200).json({
      status,
      service: 'realtime-service',
      timestamp: new Date().toISOString(),
      checks: {
        redis: redisHealth,
        nats: natsHealth,
        kafka: kafkaHealth,
      },
      connections,
      system,
    });
  }

  @Get('ready')
  async getReadiness(@Res() res: Response) {
    const [redisHealth, natsHealth] = await Promise.all([
      this.healthService.checkRedis(this.redisService),
      this.checkNats(),
    ]);
    const system = this.healthService.getSystemStats();

    const isHealthy = redisHealth.status === 'UP' && natsHealth.status === 'UP';

    res.status(isHealthy ? 200 : 503).json({
      status: isHealthy ? 'UP' : 'DOWN',
      timestamp: new Date().toISOString(),
      checks: {
        redis: redisHealth,
        nats: natsHealth,
      },
      system,
    });
  }

  @Get('metrics')
  async getMetrics(@Res() res: Response) {
    const text = await this.metricsService.getMetrics();
    res.setHeader('Content-Type', this.metricsService.getContentType());
    res.send(text);
  }

  private async checkNats() {
    const connected = this.natsService.isConnected();
    return {
      status: connected ? 'UP' : 'DOWN',
      message: connected ? undefined : 'NATS not connected',
    };
  }

  private async checkKafka() {
    const connected = this.kafkaConsumer.isConnected();
    return {
      status: connected ? 'UP' : 'DOWN',
      message: connected ? undefined : 'Kafka consumer not running',
    };
  }
}