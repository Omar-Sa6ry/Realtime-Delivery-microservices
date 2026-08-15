import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { TypeOrmModule } from '@nestjs/typeorm';
import { BullModule } from '@nestjs/bullmq';
import { GraphQLModule } from '@nestjs/graphql';
import { ApolloFederationDriver, ApolloFederationDriverConfig } from '@nestjs/apollo';
import { JwtModule } from '@nestjs/jwt';
import { RedisModule } from '@bts-soft/cache';
import { NotificationModule as BtsNotificationModule } from '@bts-soft/notifications';
import { StringValue } from 'ms';
import { AppResolver } from './app.resolver';
import { CommonModule } from './common/common.module';
import { KafkaModule } from './modules/kafka/kafka.module';
import { NotificationModule } from './modules/notification/notification.module';
import { WorkersModule } from './modules/workers/workers.module';
import { OutboxModule } from './modules/outbox/outbox.module';
import { GrpcModule } from './modules/grpc/grpc.module';
import { AuthCommonModule, LoggingModule, MetricsModule, AutomationModule } from '@delivery/common';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: ['../../.env'],
    }),

    TypeOrmModule.forRootAsync({
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        type: 'postgres',
        host: config.get<string>('DB_HOST', 'localhost'),
        port: config.get<number>('DB_PORT', 5432),
        username: config.get<string>('DB_USERNAME', 'postgres'),
        password: config.get<string>('POSTGRES_PASSWORD', 'O9M1a8r5+=2004'),
        database: config.get<string>('DB_NAME', 'delivery_notification_db'),
        entities: [__dirname + '/**/*.entity{.ts,.js}'],
        synchronize: true, // Auto-create tables (use migrations in prod)
      }),
      inject: [ConfigService],
    }),

    RedisModule,

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
      autoSchemaFile: { federation: 2 },
    }),

    JwtModule.registerAsync({
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        secret: config.get('JWT_SECRET', 'default_secret'),
        signOptions: { expiresIn: config.get('JWT_EXPIRE', '1d') as StringValue },
      }),
      inject: [ConfigService],
    }),

    AuthCommonModule.register({
      userService: { findById: async () => ({ id: 'mock', role: 'admin' }) }, // Temporary mock
    }),

    LoggingModule,
    MetricsModule,
    AutomationModule,

    CommonModule,
    KafkaModule,
    NotificationModule,
    WorkersModule,
    OutboxModule,
    GrpcModule,
  ],
  providers: [AppResolver],
})
export class AppModule {}
