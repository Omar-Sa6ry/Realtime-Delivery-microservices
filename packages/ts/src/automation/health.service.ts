import { Injectable, Logger } from '@nestjs/common';

@Injectable()
export class HealthService {
  private readonly logger = new Logger(HealthService.name);

  async checkDatabase(dataSource: any): Promise<{ status: string; message?: string }> {
    try {
      await dataSource.query('SELECT 1');
      return { status: 'UP' };
    } catch (err) {
      this.logger.error('Health Check: Database connection failed', err.stack);
      return { status: 'DOWN', message: err.message };
    }
  }

  async checkRedis(redisService: any): Promise<{ status: string; message?: string }> {
    try {
      // Handles both direct redis service or core redis service structures
      const pingResult = typeof redisService.ping === 'function' 
        ? await redisService.ping() 
        : (redisService.client && typeof redisService.client.ping === 'function'
            ? await redisService.client.ping()
            : 'PONG');
      return { status: 'UP' };
    } catch (err) {
      this.logger.error('Health Check: Redis connection failed', err.stack);
      return { status: 'DOWN', message: err.message };
    }
  }

  getSystemStats() {
    const memoryUsage = process.memoryUsage();
    return {
      uptime: process.uptime(),
      memory: {
        heapTotalMemoryMB: Math.round(memoryUsage.heapTotal / 1024 / 1024),
        heapUsedMemoryMB: Math.round(memoryUsage.heapUsed / 1024 / 1024),
        rssMB: Math.round(memoryUsage.rss / 1024 / 1024),
      },
      cpuUsage: process.cpuUsage(),
    };
  }
}
