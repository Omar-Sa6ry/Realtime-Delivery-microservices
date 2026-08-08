import { Module } from '@nestjs/common';
import { APP_GUARD } from '@nestjs/core';
import { GraphQLModule } from '@nestjs/graphql';
import { ApolloDriver, ApolloDriverConfig } from '@nestjs/apollo';
import { RateLimiter, RateLimiterAlgorithm } from '@bts-soft/validation';
// import { ApolloGatewayDriver, ApolloGatewayDriverConfig } from '@nestjs/apollo';
// import { IntrospectAndCompose, RemoteGraphQLDataSource } from '@apollo/gateway';
import { AppResolver } from './app.resolver';

@Module({
  imports: [
    GraphQLModule.forRoot<ApolloDriverConfig>({
      driver: ApolloDriver,
      autoSchemaFile: true,
      context: ({ req }: any) => {
        return { req };
      },
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

      // gateway: {
      //   supergraphSdl: new IntrospectAndCompose({
      //     subgraphs: [
      //       // { name: 'user', url: 'http://user-srv:3000/user/graphql' },
      //     ],
      //   }),

      //   buildService: ({ url }) => {
      //     return new RemoteGraphQLDataSource({
      //       url,
      //       willSendRequest({ request, context }: any) {
      //         if (context.req?.headers?.authorization) {
      //           request.http.headers.set(
      //             'authorization',
      //             context.req.headers.authorization,
      //           );
      //         }
      //         if (context.req?.headers?.['x-lang']) {
      //           request.http.headers.set(
      //             'x-lang',
      //             context.req.headers['x-lang'],
      //           );
      //         }
      //         if (context.req?.headers?.['accept-language']) {
      //           request.http.headers.set(
      //             'accept-language',
      //             context.req.headers['accept-language'],
      //           );
      //         }
      //       },
      //     });
      //   },
      // },
      
    } as ApolloDriverConfig),
  ],
  providers: [
    AppResolver,
    {
      provide: APP_GUARD,
      useClass: RateLimiter({
        algorithm: RateLimiterAlgorithm.TOKEN_BUCKET,
        limit: Number(process.env.RATE_LIMIT_LIMIT) || 100,
        windowMs: Number(process.env.RATE_LIMIT_WINDOW_MS) || 60_000,
        skipIntrospection: true,
      }),
    },
  ],
})
export class AppModule {}
