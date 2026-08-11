import { Injectable, OnModuleInit } from '@nestjs/common';
import * as client from 'prom-client';

@Injectable()
export class MetricsService implements OnModuleInit {
  private registry: client.Registry;

  public requestCounter: client.Counter<string>;
  public requestDuration: client.Histogram<string>;
  public errorCounter: client.Counter<string>;

  constructor() {
    this.registry = client.register;
  }

  onModuleInit() {
    client.collectDefaultMetrics({ register: this.registry });

    this.requestCounter = new client.Counter({
      name: 'app_requests_total',
      help: 'Total number of requests processed (HTTP, GraphQL, gRPC)',
      labelNames: ['protocol', 'method', 'path', 'statusCode'],
      registers: [this.registry],
    });

    this.requestDuration = new client.Histogram({
      name: 'app_request_duration_seconds',
      help: 'Duration of requests in seconds',
      labelNames: ['protocol', 'method', 'path', 'statusCode'],
      buckets: [0.01, 0.05, 0.1, 0.2, 0.5, 1, 2, 5],
      registers: [this.registry],
    });

    this.errorCounter = new client.Counter({
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
