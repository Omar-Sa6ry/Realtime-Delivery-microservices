import { DynamicModule, Module, Provider } from '@nestjs/common';
import { RabbitMQModuleOptions, RabbitMQService } from './rabbitmq.service';
import { RabbitMQHealthIndicator } from './rabbitmq.health';

export const RABBITMQ_MODULE_OPTIONS = 'RABBITMQ_MODULE_OPTIONS';

export interface RabbitMQModuleAsyncOptions {
  imports?: any[];
  useFactory: (...args: any[]) => RabbitMQModuleOptions | Promise<RabbitMQModuleOptions>;
  inject?: any[];
}

function buildServiceProvider(options: RabbitMQModuleOptions): Provider {
  return {
    provide: RabbitMQService,
    useFactory: () => new RabbitMQService(options),
  };
}

function resolveOptionsFromEnv(explicit?: RabbitMQModuleOptions): RabbitMQModuleOptions {
  return {
    url: explicit?.url || process.env.RABBITMQ_URL,
    serviceName:
      explicit?.serviceName ||
      process.env.RABBITMQ_CLIENT_ID ||
      process.env.SERVICE_NAME ||
      'delivery-service',
    ...(explicit?.maxReconnectAttempts !== undefined && {
      maxReconnectAttempts: explicit.maxReconnectAttempts,
    }),
    ...(explicit?.reconnectBaseDelayMs !== undefined && {
      reconnectBaseDelayMs: explicit.reconnectBaseDelayMs,
    }),
    ...(explicit?.circuitBreakerFailureThreshold !== undefined && {
      circuitBreakerFailureThreshold: explicit.circuitBreakerFailureThreshold,
    }),
    ...(explicit?.circuitBreakerResetTimeoutMs !== undefined && {
      circuitBreakerResetTimeoutMs: explicit.circuitBreakerResetTimeoutMs,
    }),
  };
}

@Module({})
export class RabbitMQModule {
  static register(options?: RabbitMQModuleOptions): DynamicModule {
    const merged = resolveOptionsFromEnv(options);
    return {
      module: RabbitMQModule,
      providers: [buildServiceProvider(merged), RabbitMQHealthIndicator],
      exports: [RabbitMQService, RabbitMQHealthIndicator],
    };
  }

  static registerAsync(asyncOptions: RabbitMQModuleAsyncOptions): DynamicModule {
    const optionsProvider: Provider = {
      provide: RABBITMQ_MODULE_OPTIONS,
      useFactory: asyncOptions.useFactory,
      inject: asyncOptions.inject || [],
    };
    const serviceProvider: Provider = {
      provide: RabbitMQService,
      useFactory: async (opts: RabbitMQModuleOptions) =>
        new RabbitMQService(resolveOptionsFromEnv(opts)),
      inject: [RABBITMQ_MODULE_OPTIONS],
    };
    return {
      module: RabbitMQModule,
      imports: asyncOptions.imports || [],
      providers: [optionsProvider, serviceProvider, RabbitMQHealthIndicator],
      exports: [RabbitMQService, RabbitMQHealthIndicator],
    };
  }
}
