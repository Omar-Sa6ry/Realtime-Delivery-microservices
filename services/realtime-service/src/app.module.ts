import { Module } from '@nestjs/common';
import { join } from 'path';
import { AppResolver } from './app.resolver';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { RedisModule } from '@bts-soft/cache';
import { GraphQLModule } from '@nestjs/graphql';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: ['../../.env'],
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
        user: req.user,
        language: req.headers['accept-language'] || 'en',
      }),
      playground: true,
      debug: false,
    }),

    RedisModule,
  ],
  providers: [AppResolver],
})
export class AppModule {}
