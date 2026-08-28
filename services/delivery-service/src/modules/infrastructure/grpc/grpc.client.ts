import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { join } from 'path';

enum CircuitState {
  CLOSED,
  OPEN,
  HALF_OPEN,
}

interface ServiceClient {
  client: grpc.Client;
  service: any; // grpc service definition
}

@Injectable()
export class GrpcClient {
  private readonly logger = new Logger(GrpcClient.name);

  private readonly deliveryService: string | undefined;
  private readonly driverServiceUrl: string | undefined;

  private delivery: ServiceClient | null = null;
  private driver: ServiceClient | null = null;

  private deliveryState: CircuitState = CircuitState.CLOSED;
  private driverState: CircuitState = CircuitState.CLOSED;
  private deliveryFailures = 0;
  private driverFailures = 0;
  private deliveryOpenedAt = 0;
  private driverOpenedAt = 0;

  constructor(config: ConfigService) {
    const cfg = config.get<RealtimeConfig>('realtime')!;
    this.deliveryService = cfg.authzDeliveryServiceUrl;
    this.driverServiceUrl = cfg.authzDriverServiceUrl;
  }


  private canAttempt(
    state: CircuitState,
    name: 'delivery' | 'driver',
  ): boolean {
    if (state === CircuitState.CLOSED || state === CircuitState.HALF_OPEN)
      return true;
    const openedAt =
      name === 'delivery' ? this.deliveryOpenedAt : this.driverOpenedAt;
    if (Date.now() - openedAt > TIMINGS.CIRCUIT_RESET_MS) {
      this.logger.log(`Circuit breaker ${name}: half-open after cool-down`);
      this.setState(name, CircuitState.HALF_OPEN);
      return true;
    }
    return false;
  }

  private async invoke(
    svc: ServiceClient,
    method: string,
    args: Record<string, unknown>,
    name: 'delivery' | 'driver',
  ): Promise<any> {
    return new Promise((resolve) => {
      const fn = svc.client[method] as any;
      if (typeof fn !== 'function') {
        this.logger.error(`gRPC method ${method} not found on ${name} service`);
        resolve(null);
        return;
      }
      const deadline = new Date(Date.now() + TIMINGS.GRPC_TIMEOUT_MS);
      fn.call(
        svc.client,
        args,
        { deadline },
        (err: grpc.ServiceError | null, resp: any) => {
          if (err) {
            this.onError(name);
            this.logger.warn(
              `gRPC ${name}.${method} error: ${err.code} ${err.details}`,
            );
            resolve(null);
            return;
          }
          this.onSuccess(name);
          resolve(resp);
        },
      );
    });
  }

  private onSuccess(name: 'delivery' | 'driver'): void {
    if (name === 'delivery') this.deliveryFailures = 0;
    else this.driverFailures = 0;
    if (this.getState(name) === CircuitState.HALF_OPEN) {
      this.setState(name, CircuitState.CLOSED);
      this.logger.log(`Circuit breaker ${name}: closed after successful probe`);
    }
  }

  private onError(name: 'delivery' | 'driver'): void {
    const failures =
      name === 'delivery' ? ++this.deliveryFailures : ++this.driverFailures;
    if (failures >= TIMINGS.CIRCUIT_FAILURE_THRESHOLD) {
      this.setState(name, CircuitState.OPEN);
      this.logger.warn(
        `Circuit breaker ${name}: OPEN after ${failures} failures`,
      );
    }
  }

  private getState(name: 'delivery' | 'driver'): CircuitState {
    return name === 'delivery' ? this.deliveryState : this.driverState;
  }

  private setState(name: 'delivery' | 'driver', state: CircuitState): void {
    if (name === 'delivery') {
      this.deliveryState = state;
      if (state === CircuitState.OPEN) this.deliveryOpenedAt = Date.now();
    } else {
      this.driverState = state;
      if (state === CircuitState.OPEN) this.driverOpenedAt = Date.now();
    }
  }

  private createClient(
    name: 'delivery' | 'driver',
    serviceName: string,
    url: string,
    protoFile: string,
  ): Promise<ServiceClient> {
    const protoPath = join(__dirname, '../../../../../protos/', protoFile);
    const packageDefinition = protoLoader.loadSync(protoPath, {
      keepCase: true,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    });
    const proto = grpc.loadPackageDefinition(packageDefinition) as any;
    const svc = proto[name][serviceName];
    const client = new svc(url, grpc.credentials.createInsecure());
    return Promise.resolve({ client, service: svc });
  }
}
