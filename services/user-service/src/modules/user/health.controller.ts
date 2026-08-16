import { Controller, Get, Res } from '@nestjs/common';
import type { Response } from 'express';
import { HealthService } from '@delivery/common';
import { DataSource } from 'typeorm';
import { RedisService } from '@bts-soft/core';

@Controller('user')
export class HealthController {
  constructor(
    private readonly healthService: HealthService,
    private readonly dataSource: DataSource,
    private readonly redisService: RedisService,
  ) {}

  /**
   * Liveness probe — always returns 200 if the process is running.
   * Used by startupProbe and livenessProbe in user-depl.yaml.
   */
  @Get('health')
  getHealth() {
    return {
      status: 'UP',
      service: 'user-service',
      timestamp: new Date().toISOString(),
      system: this.healthService.getSystemStats(),
    };
  }

  /**
   * Readiness probe — returns 503 when DB or Redis are not available.
   * Used by readinessProbe in user-depl.yaml to gate traffic until
   * dependencies are healthy.
   */
  @Get('ready')
  async getReadiness(@Res() res: Response) {
    const dbHealth = await this.healthService.checkDatabase(this.dataSource);
    const redisHealth = await this.healthService.checkRedis(this.redisService);
    const system = this.healthService.getSystemStats();

    const isHealthy = dbHealth.status === 'UP' && redisHealth.status === 'UP';

    res.status(isHealthy ? 200 : 503).json({
      status: isHealthy ? 'UP' : 'DOWN',
      timestamp: new Date().toISOString(),
      checks: {
        database: dbHealth,
        redis: redisHealth,
      },
      system,
    });
  }
}
