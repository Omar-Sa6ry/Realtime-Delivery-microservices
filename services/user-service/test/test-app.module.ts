import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { GraphQLModule } from '@nestjs/graphql';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';
import { JwtModule } from '@nestjs/jwt';
import { APP_FILTER, APP_INTERCEPTOR } from '@nestjs/core';
import { User } from '../src/common/database/entities/user.entity';
import { Address } from '../src/common/database/entities/address.entity';
import { Outbox } from '../src/common/database/entities/outbox.entity';
import { TranslationModule } from '../src/common/translation/translation.module';
import { GraphQLExceptionFilter } from '@delivery-micro/shard';
import { GraphQLResponseInterceptor } from '@delivery-micro/shard';

@Module({
  imports: [
    TranslationModule,

    JwtModule.register({
      global: true,
      secret: 'test-secret-key-for-testing',
      signOptions: { expiresIn: '1d' },
    }),

    TypeOrmModule.forRoot({
      type: 'sqlite',
      database: ':memory:',
      entities: [User, Address, Outbox],
      synchronize: true,
      logging: false,
    }),

    GraphQLModule.forRoot<ApolloFederationDriverConfig>({
      driver: ApolloFederationDriver,
      path: '/user/graphql',
      autoSchemaFile: {
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
  ],
  providers: [
    {
      provide: APP_FILTER,
      useClass: GraphQLExceptionFilter,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: GraphQLResponseInterceptor,
    },
  ],
})
export class TestAppModule {}
