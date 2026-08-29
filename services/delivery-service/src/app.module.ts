import { join } from 'path';
import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { TypeOrmModule } from '@nestjs/typeorm';
import { GraphQLModule } from '@nestjs/graphql';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';
import { APP_FILTER, APP_INTERCEPTOR } from '@nestjs/core';
import { RedisModule, NotificationModule } from '@bts-soft/core';
import { TranslationModule } from './common/translation/translation.module';
import deliveryConfig from './common/config/delivery.config';
import { DeliveryMetricsService } from './common/metrics/delivery-metrics.service';
import { DeliveryModule } from './modules/delivery/delivery.module';
import { DeliveryKafkaModule } from './modules/infrastructure/kafka/kafka.module';
import { DeliveryNatsModule } from './modules/infrastructure/nats/nats.module';
import { DeliveryGrpcModule } from './modules/infrastructure/grpc/grpc.module';

import {
  LoggingModule,
  MetricsModule,
  AutomationModule,
  MetricsInterceptor,
  GraphQLExceptionFilter,
  GraphQLResponseInterceptor,
} from '@delivery/common';
import { BullModule } from '@nestjs/bullmq';
import { HealthController } from './health.controller';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: [
        join(
          process.cwd(),
          '../../config/env/.env.' +
            (process.env.APP_ENV || process.env.NODE_ENV || 'development'),
        ),
      ],
    }),

    TranslationModule,
    RedisModule,

    TypeOrmModule.forRootAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        type: 'postgres',
        host: config.get<string>('DB_HOST', 'localhost'),
        port: Number(config.get<number>('DB_PORT', 5432)) || 5432,
        username:
          config.get<string>('DB_USERNAME') ||
          config.get<string>('POSTGRES_USER') ||
          'postgres',
        password: config.get<string>('POSTGRES_PASSWORD'),
        database: config.get<string>('DB_NAME', 'delivery_delivery_db'),
        autoLoadEntities: true,
        synchronize: true, // For development mode
        // Survive concurrent boot: retry for ~3 minutes before giving up.
        retryAttempts: 60,
        retryDelay: 3000,
      }),
    }),

    BullModule.forRootAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        connection: {
          host: config.get<string>('REDIS_HOST', 'localhost'),
          port: Number(config.get<number>('REDIS_PORT', 6379)) || 6379,
        },
      }),
    }),

    GraphQLModule.forRoot<ApolloFederationDriverConfig>({
      driver: ApolloFederationDriver,
      path: '/delivery/graphql',
      autoSchemaFile: {
        federation: 2,
      },
      context: ({ req }) => ({
        req,
        user: req.user,
        language: req.headers['accept-language'] || 'en',
      }),
      playground: true,
      debug: false,
    }),

    LoggingModule,
    MetricsModule,
    AutomationModule,
    DeliveryModule,
    DeliveryKafkaModule,
    DeliveryNatsModule,
    DeliveryGrpcModule,
  ],
  controllers: [HealthController],
  providers: [
    DeliveryMetricsService,
    {
      provide: APP_FILTER,
      useClass: GraphQLExceptionFilter,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: GraphQLResponseInterceptor,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: MetricsInterceptor,
    },
  ],
})
export class AppModule {}
