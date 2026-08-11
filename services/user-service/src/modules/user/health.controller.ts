import { Controller, Get } from '@nestjs/common';
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

  @Get('health')
  async getHealth() {
    const dbHealth = await this.healthService.checkDatabase(this.dataSource);
    const redisHealth = await this.healthService.checkRedis(this.redisService);
    const system = this.healthService.getSystemStats();

    const isHealthy = dbHealth.status === 'UP' && redisHealth.status === 'UP';

    return {
      status: isHealthy ? 'UP' : 'DOWN',
      timestamp: new Date().toISOString(),
      checks: {
        database: dbHealth,
        redis: redisHealth,
      },
      system,
    };
  }
}
