import { join } from 'path';
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
import {
  HttpExceptionFilter,
  RedisModule,
  NotificationModule,
} from '@bts-soft/core';
import { TranslationModule } from './common/translation/translation.module';
import { GraphqlResponseInterceptor } from './common/interceptors/graphql-response.interceptor';
import { CommonModule } from './common/common.module';
import { AuthModule } from './modules/auth/auth.module';
import { UserModule } from './modules/user/user.module';
import { User } from './common/database/entities/user.entity';
import { Address } from './common/database/entities/address.entity';
import { Outbox } from './common/database/entities/outbox.entity';
import { UserService } from './modules/user/user.service';
import { LoggingModule, MetricsModule, AutomationModule, MetricsInterceptor } from '@delivery/common';
import { BullModule } from '@nestjs/bullmq';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: [
        join(process.cwd(), '.env'),
        join(process.cwd(), '../../.env'),
      ],
    }),

    TranslationModule,
    RedisModule,
    NotificationModule,

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
        host:
          config.get<string>('DB_HOST', 'localhost') === 'user-db-srv' &&
          process.env.NODE_ENV !== 'production' &&
          !process.env.KUBERNETES_SERVICE_HOST
            ? 'localhost'
            : config.get<string>('DB_HOST', 'localhost'),
        port: Number(config.get<number>('DB_PORT', 5433)) || 5433,
        username:
          config.get<string>('POSTGRES_USER') ||
          config.get<string>('DB_USERNAME', 'postgres'),
        password: config.get<string>('POSTGRES_PASSWORD') || 'O9M1a8r5+=2004',
        database: config.get<string>('DB_NAME', 'delivery_user_db'),
        entities: [User, Address, Outbox],
        synchronize: true, // For development mode
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
      path: '/user/graphql',
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

    CommonModule,
    AuthModule,
    UserModule,
    LoggingModule,
    MetricsModule,
    AutomationModule,
  ],
  providers: [
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
    {
      provide: 'USER_SERVICE',
      useFactory: (userService: UserService) => {
        return {
          findById: async (id: string) => {
            const user = await userService.findById(id);
            if (!user) return null;
            return {
              id: user.id,
              email: user.email,
              role: user.role,
            };
          },
        };
      },
      inject: [UserService],
    },
  ],
})
export class AppModule {}
