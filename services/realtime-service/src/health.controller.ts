import { Controller, Get, Res } from '@nestjs/common';
import type { Response } from 'express';
import { HealthService } from '@delivery/common';
import { RedisService } from '@bts-soft/cache';

@Controller('realtime')
export class HealthController {
  constructor(
    private readonly healthService: HealthService,
    private readonly redisService: RedisService,
  ) {}

  @Get('health')
  getHealth() {
    return {
      status: 'UP',
      service: 'realtime-service',
      timestamp: new Date().toISOString(),
      system: this.healthService.getSystemStats(),
    };
  }

  @Get('ready')
  async getReadiness(@Res() res: Response) {
    const redisHealth = await this.healthService.checkRedis(this.redisService);
    const system = this.healthService.getSystemStats();

    const isHealthy = redisHealth.status === 'UP';

    res.status(isHealthy ? 200 : 503).json({
      status: isHealthy ? 'UP' : 'DOWN',
      timestamp: new Date().toISOString(),
      checks: {
        redis: redisHealth,
      },
      system,
    });
  }
}