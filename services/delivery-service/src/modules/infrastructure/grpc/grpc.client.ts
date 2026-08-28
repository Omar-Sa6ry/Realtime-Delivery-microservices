import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as grpc from '@grpc/grpc-js';

const TIMINGS = { GRPC_TIMEOUT_MS: 5000, CIRCUIT_FAILURE_THRESHOLD: 5, CIRCUIT_RESET_MS: 30000 } as const;
type ServiceName = 'delivery' | 'driver';

@Injectable()
export class GrpcClient {
  private readonly logger = new Logger(GrpcClient.name);
  private readonly urls: Record<ServiceName, string>;
  constructor(config: ConfigService) { this.urls = { delivery: config.get<string>('DELIVERY_SERVICE_GRPC_URL', 'localhost:50054'), driver: config.get<string>('DRIVER_SERVICE_GRPC_URL', 'localhost:50055') }; }
  getUrl(service: ServiceName): string { return this.urls[service]; }
  getTimeout(): number { return TIMINGS.GRPC_TIMEOUT_MS; }
  createClient<T extends grpc.Client>(service: ServiceName, Service: new (address: string, credentials: grpc.ChannelCredentials) => T): T { this.logger.debug(`Creating gRPC client for ${service} at ${this.urls[service]}`); return new Service(this.urls[service], grpc.credentials.createInsecure()); }
}
