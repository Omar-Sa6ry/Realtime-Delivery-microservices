import { Module, MiddlewareConsumer, NestModule } from '@nestjs/common';
import { APP_GUARD, APP_INTERCEPTOR } from '@nestjs/core';
import { ConfigModule } from '@nestjs/config';
import { join } from 'path';
import { JwtModule } from '@nestjs/jwt';
import { GraphQLModule } from '@nestjs/graphql';
import { ApolloDriverConfig } from '@nestjs/apollo';
import { RedisModule, RedisService } from '@bts-soft/cache';
import { AppController } from './app.controller';
import { JwtAuthGuard } from './common/guards/auth.guard';
import { createRateLimiterGuard } from './common/guards/rate-limiter.factory';
import { HealthController } from './health/health.controller';
import { MetricsController } from './health/metrics.controller';
import {
  LoggingModule,
  MetricsModule,
  AutomationModule,
  MetricsInterceptor,
} from '@delivery-micro/shard';
import depthLimit from 'graphql-depth-limit';
import { CorrelationIdMiddleware } from './common/middlewares/correlation-id.middleware';
import { JwtAuthMiddleware } from './common/middlewares/jwt-auth.middleware';
import { ApolloGatewayDriver, ApolloGatewayDriverConfig } from '@nestjs/apollo';
import { IntrospectAndCompose } from '@apollo/gateway';
import { buildGraphQLContext } from './common/graphql/graphql-context';
import { formatGatewayError } from './common/graphql/format-error';
import { buildSubgraphService } from './common/graphql/subgraph-service';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: [
        join(process.cwd(), '.env'),
        join(process.cwd(), '../../config/.env'),
      ],
    }),

    JwtModule.register({
      secret: process.env.JWT_SECRET || 'super-secret-jwt-key',
    }),

    RedisModule,

    GraphQLModule.forRoot<ApolloDriverConfig>({
      driver: ApolloGatewayDriver,
      server: {
        context: buildGraphQLContext,
        validationRules: [depthLimit(7)],
        playground: true,
        introspection: true,
        formatError: formatGatewayError,
      },
      gateway: {
        supergraphSdl: new IntrospectAndCompose({
          subgraphs: [
            {
              name: 'media',
              url:
                process.env.MEDIA_SUBGRAPH_URL ||
                'http://media-srv:4005/media/graphql',
            },
            {
              name: 'notification',
              url:
                process.env.NOTIFICATION_SERVICE_URL ||
                'http://notification-srv:4004/notification/graphql',
            },
            {
              name: 'realtime',
              url:
                process.env.REALTIME_SERVICE_URL ||
                'http://realtime-srv:4006/realtime/graphql',
            },
            {
              name: 'search',
              url:
                process.env.SEARCH_SERVICE_URL ||
                'http://search-srv:4007/search/graphql',
            },
            {
              name: 'payment',
              url:
                process.env.PAYMENT_SUBGRAPH_URL ||
                'http://payment-srv:4002/payment/graphql',
            },
            {
              name: 'user',
              url:
                process.env.USER_SERVICE_URL ||
                'http://user-srv:4001/user/graphql',
            },
            {
              name: 'delivery',
              url:
                process.env.DELIVERY_SERVICE_URL ||
                'http://delivery-srv:4003/delivery/graphql',
            },
            {
              name: 'driver',
              url:
                process.env.DRIVER_SERVICE_URL ||
                'http://driver-srv:4008/driver/graphql',
            },
            {
              name: 'analytics',
              url:
                process.env.ANALYTICS_SERVICE_URL ||
                'http://analytics-srv:4009/analytics/graphql',
            },
          ],
          pollIntervalInMs: 5000,
        }),
        buildService: buildSubgraphService,
      },
    } as ApolloGatewayDriverConfig),
    LoggingModule,
    MetricsModule,
    AutomationModule,
  ],
  controllers: [HealthController, MetricsController, AppController],
  providers: [
    {
      provide: APP_GUARD,
      useClass: JwtAuthGuard,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: MetricsInterceptor,
    },

    {
      provide: APP_GUARD,
      useFactory: createRateLimiterGuard,
      inject: [RedisService],
    },
  ],
})
export class AppModule implements NestModule {
  configure(consumer: MiddlewareConsumer) {
    consumer.apply(CorrelationIdMiddleware, JwtAuthMiddleware).forRoutes('*');
  }
}
