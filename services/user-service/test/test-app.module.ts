import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { TypeOrmModule } from '@nestjs/typeorm';
import { GraphQLModule } from '@nestjs/graphql';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';
import { JwtModule } from '@nestjs/jwt';
import { APP_FILTER, APP_INTERCEPTOR } from '@nestjs/core';
import { RedisService, NotificationService } from '@bts-soft/core';
import {
  GraphQLExceptionFilter,
  GraphQLResponseInterceptor,
  MetricsInterceptor,
  HealthService,
  MetricsService,
  KafkaService,
} from '@delivery-micro/shard';

import { User } from '../src/common/database/entities/user.entity';
import { Address } from '../src/common/database/entities/address.entity';
import { Outbox } from '../src/common/database/entities/outbox.entity';
import { TranslationModule } from '../src/common/translation/translation.module';
import { BcryptPasswordHasher } from '../src/common/security/bcrypt-password.hasher';
import { JwtTokenProvider } from '../src/common/security/jwt-token.provider';
import { RedisSessionRepository } from '../src/common/database/repositories/redis-session.repository';
import { OutboxWorkerService } from '../src/common/messaging/outbox-worker.service';

import { AppResolver } from '../src/app.resolver';
import { AuthService } from '../src/modules/auth/auth.service';
import { AuthResolver } from '../src/modules/auth/auth.resolver';
import { UserFactory } from '../src/modules/auth/user.factory';
import { DbUserService } from '../src/modules/user/db-user.service';
import { UserService } from '../src/modules/user/user.service';
import { UserResolver } from '../src/modules/user/user.resolver';
import { UserGrpcController } from '../src/modules/user/user-grpc.controller';
import { HealthController } from '../src/modules/user/health.controller';
import { MetricsController } from '../src/modules/user/metrics.controller';
import { MediaGrpcService } from '../src/modules/media/grpc-media.service';
import { UserRabbitMQPublisher } from '../src/modules/rabbitmq/rabbitmq.publisher';

import { createFakeRedis } from './support/fake-redis';
import {
  createKafkaFake,
  createNotificationFake,
  createOutboxFake,
  createRabbitPublisherFake,
  createMediaFake,
} from './support/fakes';

/**
 * End-to-end application module.
 *
 * It is a faithful copy of `src/app.module.ts` with two deliberate
 * differences:
 *
 *  1. PostgreSQL is pointed at the `E2E_DB_*` database (see
 *     `test/support/test-env.ts`) instead of the developer database, and the
 *     BullMQ/Redis/Kafka/RabbitMQ/NATS transports are replaced by in-memory
 *     doubles (`test/support/fakes.ts`, `test/support/fake-redis.ts`). Only
 *     Postgres is required to run the suite.
 *  2. `ConfigModule` ignores `.env` files so a developer's local environment
 *     can never redirect the suite.
 *
 * Everything else — resolvers, controllers, guards (`RoleGuard`,
 * `RedisRateLimiter`), the federation driver, i18n, the response interceptor
 * and the exception filter — is the production wiring.
 */
@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      ignoreEnvFile: true,
    }),

    TranslationModule,

    JwtModule.registerAsync({
      global: true,
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        secret: config.get<string>('JWT_SECRET') || 'default_secret',
        signOptions: { expiresIn: '1d' },
      }),
    }),

    TypeOrmModule.forRootAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        type: 'postgres',
        host: config.get<string>('DB_HOST', 'localhost'),
        port: Number(config.get<number>('DB_PORT', 5433)) || 5433,
        username:
          config.get<string>('POSTGRES_USER') ||
          config.get<string>('DB_USERNAME', 'postgres'),
        password: config.get<string>('POSTGRES_PASSWORD'),
        database: config.get<string>('DB_NAME', 'delivery_user_db_e2e'),
        entities: [User, Address, Outbox],
        synchronize: true,
        logging: false,
        // Fail fast instead of the 3-minute production boot retry loop.
        retryAttempts: 5,
        retryDelay: 2000,
      }),
    }),

    TypeOrmModule.forFeature([User, Address, Outbox]),

    GraphQLModule.forRoot<ApolloFederationDriverConfig>({
      driver: ApolloFederationDriver,
      path: '/user/graphql',
      autoSchemaFile: {
        federation: 2,
      },
      context: ({ req, res }) => ({
        req,
        res,
        user: req.user,
        language: req.headers['accept-language'] || 'en',
      }),
      playground: true,
      debug: false,
    }),
  ],

  controllers: [HealthController, MetricsController, UserGrpcController],

  providers: [
    AppResolver,

    // src/common/common.module.ts providers
    BcryptPasswordHasher,
    JwtTokenProvider,
    RedisSessionRepository,

    // src/modules/auth/auth.module.ts providers
    UserFactory,
    AuthService,
    AuthResolver,

    // src/modules/user/user.module.ts providers
    DbUserService,
    UserService,
    UserResolver,
    {
      provide: 'USER_SERVICE',
      useFactory: (userService: UserService) => ({
        findById: async (id: string) => {
          const user = await userService.findById(id);
          if (!user) return null;
          return { id: user.id, email: user.email, role: user.role };
        },
      }),
      inject: [UserService],
    },

    // src/common and src/modules infrastructure providers
    HealthService,
    MetricsService,

    // ---- in-memory doubles for external integrations ----
    {
      provide: RedisService,
      useFactory: () => createFakeRedis(),
    },
    {
      provide: 'SHARED_REDIS_SERVICE',
      useExisting: RedisService,
    },
    {
      provide: NotificationService,
      useFactory: () => createNotificationFake(),
    },
    {
      provide: KafkaService,
      useFactory: () => createKafkaFake(),
    },
    {
      provide: OutboxWorkerService,
      useFactory: () => createOutboxFake(),
    },
    {
      provide: MediaGrpcService,
      useFactory: () => createMediaFake(),
    },
    {
      provide: UserRabbitMQPublisher,
      useFactory: () => createRabbitPublisherFake(),
    },

    // ---- global pipeline (same as src/app.module.ts) ----
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
export class TestAppModule {}
