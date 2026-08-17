import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { APP_FILTER, APP_INTERCEPTOR } from '@nestjs/core';
import { join } from 'path';
import { GraphQLModule } from '@nestjs/graphql';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';
import { JwtModule } from '@nestjs/jwt';
import { StringValue } from 'ms';
import { RedisModule } from '@bts-soft/cache';
import {
  HttpExceptionFilter,
} from '@bts-soft/core';
import { GraphqlResponseInterceptor } from './common/interceptors/graphql-response.interceptor';
import { WebSocketModule } from './modules/gateway/websocket/websocket.module';
import { ConnectionModule } from './modules/gateway/connection/connection.module';
import { SubscriptionModule } from './modules/features/subscription/subscription.module';
import { PresenceModule } from './modules/features/presence/presence.module';
import { LocationModule } from './modules/features/location/location.module';
import { HeartbeatModule } from './modules/gateway/heartbeat/heartbeat.module';
import { AuthorizationModule } from './modules/gateway/authorization/authorization.module';
import { EventsModule } from './modules/features/events/events.module';
import { NatsModule } from './modules/infrastructure/nats/nats.module';
import { KafkaConsumerModule } from './modules/infrastructure/kafka/kafka.module';
import { GrpcModule } from './modules/infrastructure/grpc/grpc.module';
import { CommandModule } from './modules/features/command/command.module';
import { AuthModule } from './modules/gateway/auth/auth.module';
import {
  LoggingModule,
  MetricsModule,
  AutomationModule,
  MetricsInterceptor,
} from '@delivery/common';
import { CommonModule } from './common/common.module';
import realtimeConfig from './common/config/realtime.config';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      load: [realtimeConfig],
      envFilePath: ['../../.env'],
    }),

    RedisModule,

    JwtModule.registerAsync({
      global: true,
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        secret: config.get<string>('JWT_SECRET') || 'default_secret',
        signOptions: { expiresIn: config.get<string>('JWT_EXPIRE', '36000s') as StringValue },
      }),
      inject: [ConfigService],
    }),

    GraphQLModule.forRoot<ApolloFederationDriverConfig>({
      driver: ApolloFederationDriver,
      path: '/realtime/graphql',
      autoSchemaFile: {
        path: join(process.cwd(), 'src/schema.gql'),
        federation: 2,
      },
      context: ({ req }) => ({
        req,
        user: req?.user,
        language: req?.headers?.['accept-language'] || 'en',
      }),
      playground: true,
      debug: false,
    }),

    LoggingModule,
    MetricsModule,
    AutomationModule,

    // Core realtime infrastructure
    NatsModule,
    EventsModule,
    KafkaConsumerModule,
    GrpcModule,
    AuthModule,

    // Domains
    ConnectionModule,
    SubscriptionModule,
    PresenceModule,
    LocationModule,
    HeartbeatModule,
    AuthorizationModule,
    CommandModule,
    WebSocketModule,

    CommonModule,
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
  ],
})
export class AppModule {}