import { Injectable, OnModuleInit } from '@nestjs/common';
import * as client from 'prom-client';

@Injectable()
export class MetricsService implements OnModuleInit {
  private static defaultMetricsRegistered = false;

  private registry: client.Registry;

  public requestCounter: client.Counter<string>;
  public requestDuration: client.Histogram<string>;
  public errorCounter: client.Counter<string>;

  constructor() {
    this.registry = client.register;
  }

  onModuleInit() {
    if (!MetricsService.defaultMetricsRegistered) {
      client.collectDefaultMetrics({ register: this.registry });
      MetricsService.defaultMetricsRegistered = true;
    }

    const existingCounter = this.registry.getSingleMetric(
      'app_requests_total',
    ) as client.Counter<string> | undefined;
    const existingDuration = this.registry.getSingleMetric(
      'app_request_duration_seconds',
    ) as client.Histogram<string> | undefined;
    const existingErrors = this.registry.getSingleMetric(
      'app_errors_total',
    ) as client.Counter<string> | undefined;

    this.requestCounter =
      existingCounter ||
      new client.Counter({
        name: 'app_requests_total',
        help: 'Total number of requests processed (HTTP, GraphQL, gRPC)',
        labelNames: ['protocol', 'method', 'path', 'statusCode'],
        registers: [this.registry],
      });

    this.requestDuration =
      existingDuration ||
      new client.Histogram({
        name: 'app_request_duration_seconds',
        help: 'Duration of requests in seconds',
        labelNames: ['protocol', 'method', 'path', 'statusCode'],
        buckets: [0.01, 0.05, 0.1, 0.2, 0.5, 1, 2, 5],
        registers: [this.registry],
      });

    this.errorCounter =
      existingErrors ||
      new client.Counter({
        name: 'app_errors_total',
        help: 'Total number of application errors',
        labelNames: ['context', 'errorCode'],
        registers: [this.registry],
      });
  }

  async getMetrics(): Promise<string> {
    return this.registry.metrics();
  }

  getContentType(): string {
    return this.registry.contentType;
  }
}
