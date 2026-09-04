import { Module, MiddlewareConsumer, NestModule } from '@nestjs/common';
import { APP_GUARD, APP_INTERCEPTOR } from '@nestjs/core';
import { ConfigModule } from '@nestjs/config';
import { join } from 'path';
import { JwtModule } from '@nestjs/jwt';
import { GraphQLModule } from '@nestjs/graphql';
import { ApolloDriver, ApolloDriverConfig } from '@nestjs/apollo';
import { RedisModule, RedisService } from '@bts-soft/cache';
import {
  RateLimiter,
  RateLimiterAlgorithm,
  RedisStore,
} from '@bts-soft/validation';
import { AppResolver } from './app.resolver';
import { JwtAuthGuard } from './common/guards/auth.guard';
import { HealthController } from './health/health.controller';
import { MetricsController } from './health/metrics.controller';
import {
  LoggingModule,
  MetricsModule,
  AutomationModule,
  MetricsInterceptor,
} from '@delivery/common';
import depthLimit from 'graphql-depth-limit';
import {
  CorrelationIdMiddleware,
  CORRELATION_ID_HEADER,
} from './common/middlewares/correlation-id.middleware';
import { JwtAuthMiddleware } from './common/middlewares/jwt-auth.middleware';
import { ApolloGatewayDriver, ApolloGatewayDriverConfig } from '@nestjs/apollo';
import { IntrospectAndCompose, RemoteGraphQLDataSource } from '@apollo/gateway';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: [join(process.cwd(), '.env'), join(process.cwd(), '../../config/.env')],
    }),

    JwtModule.register({
      secret: process.env.JWT_SECRET || 'super-secret-jwt-key',
    }),

    RedisModule,

    GraphQLModule.forRoot<ApolloDriverConfig>({
      driver: ApolloGatewayDriver,
      server: {
        context: ({ req }: any) => {
          if (req && !req.user && req.headers?.authorization?.startsWith('Bearer ')) {
            try {
              const token = req.headers.authorization.split(' ')[1];
              const parts = token.split('.');
              if (parts.length === 3) {
                const payload = JSON.parse(Buffer.from(parts[1], 'base64').toString('utf-8'));
                req.user = {
                  userId: payload.sub || payload.userId || payload.id,
                  role: payload.role || 'USER',
                  sessionId: payload.sessionId,
                };
              }
            } catch {
              // ignore invalid format
            }
          }
          return { req };
        },
        validationRules: [depthLimit(7)],
        playground: true,
        introspection: true,
        formatError: (error: any) => {
          // Handle subgraph errors (new format: extensions directly on error)
          const subgraphError = error.extensions;

          if (subgraphError && subgraphError.success === false && subgraphError.statusCode) {
            return {
              success: false,
              statusCode: subgraphError.statusCode,
              message: error.message,
              timeStamp: subgraphError.timeStamp || new Date().toISOString(),
              error: subgraphError.error,
            } as any;
          }

          // Handle legacy subgraph error format (if any)
          const legacySubgraphError = error.extensions?.response?.body?.errors?.[0];
          if (legacySubgraphError) {
            return {
              success: false,
              statusCode: legacySubgraphError.statusCode || 400,
              message: legacySubgraphError.message,
              timeStamp: legacySubgraphError.timeStamp || new Date().toISOString(),
            } as any;
          }

          // Handle gateway-level errors
          const originalError = error.extensions?.originalError as any;
          const msg = originalError?.message || error.message;
          const code =
            error.extensions?.statusCode || originalError?.statusCode || 400;

          return {
            success: false,
            statusCode: code,
            message: Array.isArray(msg) ? msg[0] : msg,
            timeStamp: new Date().toISOString(),
          } as any;
        },
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
              name: 'user',
              url:
                process.env.USER_SERVICE_URL ||
                'http://user-srv:4001/user/graphql',
            },     {
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
          ],
          pollIntervalInMs: 5000,
        }),
        buildService: ({ url }) => {
          return new RemoteGraphQLDataSource({
            url,
            willSendRequest({ request, context }: any) {
              let user = context.req?.user;
              if (!user && context.req?.headers?.authorization?.startsWith('Bearer ')) {
                try {
                  const token = context.req.headers.authorization.split(' ')[1];
                  const parts = token.split('.');
                  if (parts.length === 3) {
                    const payload = JSON.parse(Buffer.from(parts[1], 'base64').toString('utf-8'));
                    user = {
                      userId: payload.sub || payload.userId || payload.id,
                      role: payload.role || 'USER',
                      sessionId: payload.sessionId,
                    };
                  }
                } catch {
                  // ignore
                }
              }

              if (user) {
                request.http.headers.set(
                  'x-user-id',
                  user.userId || '',
                );
                request.http.headers.set(
                  'x-user-role',
                  user.role || '',
                );
                if (user.sessionId) {
                  request.http.headers.set(
                    'x-user-session',
                    user.sessionId,
                  );
                }
              }
              if (context.req?.headers?.[CORRELATION_ID_HEADER]) {
                request.http.headers.set(
                  CORRELATION_ID_HEADER,
                  context.req.headers[CORRELATION_ID_HEADER],
                );
              }
              if (context.req?.headers?.authorization) {
                request.http.headers.set(
                  'authorization',
                  context.req.headers.authorization,
                );
              }
            },
          });
        },
      },
    } as ApolloGatewayDriverConfig),
    LoggingModule,
    MetricsModule,
    AutomationModule,
  ],
  controllers: [HealthController, MetricsController],
  providers: [
    AppResolver,

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
      useFactory: (redisService: RedisService) => {
        return new ((RateLimiter as any)(
          {
            algorithm: RateLimiterAlgorithm.TOKEN_BUCKET,
            limit: Number(process.env.RATE_LIMIT_LIMIT) || 100,
            windowMs: Number(process.env.RATE_LIMIT_WINDOW_MS) || 60_000,
            skipIntrospection: true,
            store: new RedisStore(redisService),
          },
          new RedisStore(redisService),
        ))();
      },
      inject: [RedisService],
    },
  ],
})
export class AppModule implements NestModule {
  configure(consumer: MiddlewareConsumer) {
    consumer.apply(CorrelationIdMiddleware, JwtAuthMiddleware).forRoutes('*');
  }
}
