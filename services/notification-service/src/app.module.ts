import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { APP_FILTER, APP_INTERCEPTOR } from '@nestjs/core';
import { TypeOrmModule } from '@nestjs/typeorm';
import { BullModule } from '@nestjs/bullmq';
import { GraphQLModule } from '@nestjs/graphql';
import { ApolloFederationDriver, ApolloFederationDriverConfig } from '@nestjs/apollo';
import { JwtModule } from '@nestjs/jwt';
import { RedisModule } from '@bts-soft/cache';
import { NotificationModule as BtsNotificationModule, HttpExceptionFilter } from '@bts-soft/core';
import { StringValue } from 'ms';
import { join } from 'path';
import { AppResolver } from './app.resolver';
import { CommonModule } from './common/common.module';
import { TranslationModule } from './common/translation/translation.module';
import { GraphqlResponseInterceptor } from './common/interceptors/graphql-response.interceptor';
import { KafkaModule } from '@delivery/common';
import { KafkaConsumerModule } from './modules/kafka/kafka.module';
import { NotificationModule } from './modules/notification/notification.module';
import { WorkersModule } from './modules/workers/workers.module';
import { OutboxModule } from './modules/outbox/outbox.module';
import { GrpcModule } from './modules/grpc/grpc.module';
import { AuthModule } from './modules/auth/auth.module';
import { LoggingModule, MetricsModule, AutomationModule, MetricsInterceptor } from '@delivery/common';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: ['../../.env'],
    }),

    TranslationModule,

    RedisModule,

    JwtModule.registerAsync({
      global: true,
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        secret: config.get<string>('JWT_SECRET') || 'default_secret',
        signOptions: { expiresIn: config.get<string>('JWT_EXPIRE', '1d') as StringValue },
      }),
      inject: [ConfigService],
    }),

    TypeOrmModule.forRootAsync({
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        type: 'postgres',
        host: config.get<string>('DB_HOST', 'localhost'),
        port: config.get<number>('DB_PORT', 5432),
        username: config.get<string>('DB_USERNAME', 'postgres'),
        password: config.get<string>('POSTGRES_PASSWORD'),
        database: config.get<string>('DB_NAME', 'delivery_notification_db'),
        entities: [__dirname + '/**/*.entity{.ts,.js}'],
        synchronize: true, // Auto-create tables (use migrations in prod)
        retryAttempts: 60,
        retryDelay: 3000,
      }),
      inject: [ConfigService],
    }),

    BullModule.forRootAsync({
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        connection: {
          host: config.get('REDIS_HOST', 'localhost'),
          port: config.get('REDIS_PORT', 6379),
          db: config.get('REDIS_DB', 0),
        },
      }),
      inject: [ConfigService],
    }),

    BtsNotificationModule,

    GraphQLModule.forRoot<ApolloFederationDriverConfig>({
      driver: ApolloFederationDriver,
      path: '/notification/graphql',
      autoSchemaFile: {
        path: join(process.cwd(), 'src/schema.gql'),
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

    KafkaModule.registerAsync({
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        clientId: 'notification-service',
        brokers: (config.get<string>('KAFKA_BROKERS', 'kafka-srv:9092') || '')
          .split(',').map(b => b.trim()).filter(Boolean),
      }),
      inject: [ConfigService],
    }),

    AuthModule,
    CommonModule,
    KafkaConsumerModule,
    NotificationModule,
    WorkersModule,
    OutboxModule,
    GrpcModule,
  ],
  providers: [
    AppResolver,
    {
      provide: APP_FILTER,
      useClass: HttpExceptionFilter,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: GraphqlResponseInterceptor,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: MetricsInterceptor,
    },
  ],
})
export class AppModule {}