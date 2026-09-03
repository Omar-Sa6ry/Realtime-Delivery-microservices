import { Controller, Get, Header, HttpCode, Res } from '@nestjs/common';
import type { Response } from 'express';
import { DeliveryMetricsService } from './common/metrics/delivery-metrics.service';

@Controller()
export class HealthController {
  constructor(private readonly deliveryMetrics: DeliveryMetricsService) {}

  @Get(['health', 'delivery/health'])
  @HttpCode(200)
  getHealth() {
    return {
      status: 'UP',
      service: 'delivery-service',
      timestamp: new Date().toISOString(),
    };
  }

  @Get(['health/live', 'delivery/health/live', 'delivery/live'])
  @HttpCode(200)
  getLiveness() {
    return { status: 'UP' };
  }

  @Get(['health/ready', 'delivery/health/ready', 'delivery/ready'])
  @HttpCode(200)
  getReadiness() {
    return { status: 'UP' };
  }

  @Get(['metrics', 'delivery/metrics'])
  @Header('Content-Type', 'text/plain; version=0.0.4; charset=utf-8')
  async getMetrics(@Res() res: Response) {
    res
      .type(this.deliveryMetrics.getContentType())
      .send(await this.deliveryMetrics.getMetrics());
  }
}
