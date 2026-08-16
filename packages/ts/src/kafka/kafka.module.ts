import { DynamicModule, Module, Provider } from '@nestjs/common';
import { KafkaModuleOptions, KafkaService } from './kafka.service';

export interface KafkaModuleAsyncOptions {
  imports?: any[];
  useFactory: (...args: any[]) => KafkaModuleOptions | Promise<KafkaModuleOptions>;
  inject?: any[];
}

@Module({})
export class KafkaModule {
  static register(options?: KafkaModuleOptions): DynamicModule {
    const merged: KafkaModuleOptions = {
      clientId: options?.clientId || process.env.KAFKA_CLIENT_ID || 'delivery-service',
      brokers: options?.brokers?.length
        ? options.brokers
        : (process.env.KAFKA_BROKERS || 'localhost:9092').split(',').map(b => b.trim()).filter(Boolean),
      ...(options?.ssl !== undefined && { ssl: options.ssl }),
      ...(options?.sasl !== undefined && { sasl: options.sasl }),
      ...(options?.connectionTimeout !== undefined && { connectionTimeout: options.connectionTimeout }),
    };

    const provider: Provider = {
      provide: KafkaService,
      useFactory: () => new KafkaService(merged),
    };

    return {
      module: KafkaModule,
      providers: [provider],
      exports: [KafkaService],
    };
  }

  static registerAsync(asyncOptions: KafkaModuleAsyncOptions): DynamicModule {
    const provider: Provider = {
      provide: KafkaService,
      useFactory: async (...args: any[]) => new KafkaService(await asyncOptions.useFactory(...args)),
      inject: asyncOptions.inject || [],
    };

    return {
      module: KafkaModule,
      imports: asyncOptions.imports || [],
      providers: [provider],
      exports: [KafkaService],
    };
  }
}