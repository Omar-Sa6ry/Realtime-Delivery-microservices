import { Module, MiddlewareConsumer, NestModule } from '@nestjs/common';
import { APP_GUARD, APP_INTERCEPTOR } from '@nestjs/core';
import { ConfigModule } from '@nestjs/config';
import { JwtModule } from '@nestjs/jwt';
import { GraphQLModule } from '@nestjs/graphql';
import { ApolloDriver, ApolloDriverConfig } from '@nestjs/apollo';
import { RedisModule, RedisService } from '@bts-soft/cache';
import { RateLimiter, RateLimiterAlgorithm, RedisStore } from '@bts-soft/validation';
import { AppResolver } from './app.resolver';
import { JwtAuthGuard } from './common/guards/auth.guard';
import { HealthController } from './health/health.controller';
import { MetricsController } from './health/metrics.controller';
import { LoggingModule, MetricsModule, AutomationModule, MetricsInterceptor } from '@delivery/common';
import depthLimit from 'graphql-depth-limit';
import {
  CorrelationIdMiddleware,
  CORRELATION_ID_HEADER,
} from './common/middlewares/correlation-id.middleware';
import { ApolloGatewayDriver, ApolloGatewayDriverConfig } from '@nestjs/apollo';
import { IntrospectAndCompose, RemoteGraphQLDataSource } from '@apollo/gateway';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
    }),

    JwtModule.register({
      secret: process.env.JWT_SECRET || 'super-secret-jwt-key',
    }),

    RedisModule,

    GraphQLModule.forRoot<ApolloDriverConfig>({
      driver: ApolloGatewayDriver,
      server: {
        context: ({ req }: any) => {
          return { req };
        },
        validationRules: [depthLimit(7)],
        playground: true,
        introspection: true,
        formatError: (error: any) => {
          const subgraphError = error.extensions?.response?.body?.errors?.[0];

          if (subgraphError) {
            return {
              success: false,
              statusCode: subgraphError.statusCode || 400,
              message: subgraphError.message,
              timeStamp: subgraphError.timeStamp || new Date().toISOString(),
            } as any;
          }

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
            { name: 'user', url: process.env.USER_SERVICE_URL || 'http://user-srv:4001/user/graphql' },
          ],
        }),
        buildService: ({ url }) => {
          return new RemoteGraphQLDataSource({
            url,
            willSendRequest({ request, context }: any) {
              // Header Propagation: Inject user identity & correlation headers to downstream subgraphs
              if (context.req?.user) {
                request.http.headers.set('x-user-id', context.req.user.userId || '');
                request.http.headers.set('x-user-role', context.req.user.role || '');
                if (context.req.user.sessionId) {
                  request.http.headers.set('x-user-session', context.req.user.sessionId);
                }
              }
              if (context.req?.headers?.[CORRELATION_ID_HEADER]) {
                request.http.headers.set(
                  CORRELATION_ID_HEADER,
                  context.req.headers[CORRELATION_ID_HEADER],
                );
              }
              if (context.req?.headers?.authorization) {
                request.http.headers.set('authorization', context.req.headers.authorization);
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
    consumer.apply(CorrelationIdMiddleware).forRoutes('*');
  }
}
