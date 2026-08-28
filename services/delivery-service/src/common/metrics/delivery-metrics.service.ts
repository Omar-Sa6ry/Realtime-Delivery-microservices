import { Injectable } from '@nestjs/common';
import { MetricsService } from '@delivery/common';

@Injectable()
export class DeliveryMetricsService {
  constructor(private readonly metrics: MetricsService) {}
  getMetrics(): Promise<string> { return this.metrics.getMetrics(); }
  getContentType(): string { return this.metrics.getContentType(); }
}
