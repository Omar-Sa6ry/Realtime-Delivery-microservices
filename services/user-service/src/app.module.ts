import { join } from 'path';
import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { CqrsModule } from '@nestjs/cqrs';
import { TypeOrmModule } from '@nestjs/typeorm';
import { GraphQLModule } from '@nestjs/graphql';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';
import { JwtModule } from '@nestjs/jwt';
import { APP_FILTER } from '@nestjs/core';
import { HttpExceptionFilter, RedisModule, NotificationModule } from '@bts-soft/core';
import { TranslationModule } from './common/translation/translation.module';
import { UserFactory } from './domain/factories/user.factory';
import { UserOrmEntity } from './infrastructure/database/entities/user.orm-entity';
import { AddressOrmEntity } from './infrastructure/database/entities/address.orm-entity';
import { OutboxOrmEntity } from './infrastructure/database/entities/outbox.orm-entity';
import { IUSER_REPOSITORY } from './domain/repositories/user.repository.interface';
import { ISESSION_REPOSITORY } from './domain/repositories/session.repository.interface';
import { IPASSWORD_HASHER } from './application/ports/password-hasher.port';
import { ITOKEN_PROVIDER } from './application/ports/token-provider.port';
import { TypeOrmUserRepository } from './infrastructure/database/repositories/typeorm-user.repository';
import { RedisSessionRepository } from './infrastructure/database/repositories/redis-session.repository';
import { BcryptPasswordHasher } from './infrastructure/security/bcrypt-password.hasher';
import { JwtTokenProvider } from './infrastructure/security/jwt-token.provider';
import { OutboxWorkerService } from './infrastructure/messaging/outbox-worker.service';
import { BullModule } from '@nestjs/bullmq';
import { OutboxProcessor } from './infrastructure/messaging/outbox.queue';
import { RegisterUserHandler } from './application/commands/register-user/register-user.handler';
import { LoginUserHandler } from './application/commands/login-user/login-user.handler';
import { GetUserByIdHandler } from './application/queries/get-user-by-id/get-user-by-id.handler';
import { ForgetPasswordHandler } from './application/commands/forget-password/forget-password.handler';
import { ResetPasswordHandler } from './application/commands/reset-password/reset-password.handler';
import { ChangePasswordHandler } from './application/commands/change-password/change-password.handler';
import { LogoutHandler } from './application/commands/logout/logout.handler';
import { FindUsersHandler } from './application/queries/find-users/find-users.handler';
import { RefreshTokenHandler } from './application/commands/refresh-token/refresh-token.handler';
import { UpdateProfileHandler } from './application/commands/update-profile/update-profile.handler';
import { AuthResolver } from './presentation/graphql/resolvers/auth.resolver';
import { UserResolver } from './presentation/graphql/resolvers/user.resolver';
import { UserGrpcController } from './presentation/grpc/user-grpc.controller';

const CommandHandlers = [
  RegisterUserHandler,
  LoginUserHandler,
  ForgetPasswordHandler,
  ResetPasswordHandler,
  ChangePasswordHandler,
  LogoutHandler,
  RefreshTokenHandler,
  UpdateProfileHandler,
];
const QueryHandlers = [
  GetUserByIdHandler,
  FindUsersHandler,
];

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: [join(process.cwd(), '.env'), join(process.cwd(), '../../.env')],
    }),
    
    CqrsModule,
    TranslationModule,
    RedisModule,
    NotificationModule,

    JwtModule.registerAsync({
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
        host: config.get<string>('DB_HOST', 'localhost') === 'user-db-srv' && process.env.NODE_ENV !== 'production' && !process.env.KUBERNETES_SERVICE_HOST ? 'localhost' : config.get<string>('DB_HOST', 'localhost'),
        port: Number(config.get<number>('DB_PORT', 5433)) || 5433,
        username: config.get<string>('POSTGRES_USER') || config.get<string>('DB_USERNAME', 'postgres'),
        password: config.get<string>('POSTGRES_PASSWORD') || 'O9M1a8r5+=2004',
        database: config.get<string>('DB_NAME', 'delivery_user_db'),
        entities: [UserOrmEntity, AddressOrmEntity, OutboxOrmEntity],
        synchronize: true, // For development mode
      }),
    }),

    TypeOrmModule.forFeature([UserOrmEntity, AddressOrmEntity, OutboxOrmEntity]),

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
    BullModule.registerQueue({
      name: 'outbox-queue',
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
  ],
  controllers: [UserGrpcController],
  providers: [
    UserFactory,
    OutboxWorkerService,
    OutboxProcessor,
    ...CommandHandlers,
    ...QueryHandlers,
    AuthResolver,
    UserResolver,
    {
      provide: IUSER_REPOSITORY,
      useClass: TypeOrmUserRepository,
    },
    {
      provide: ISESSION_REPOSITORY,
      useClass: RedisSessionRepository,
    },
    {
      provide: IPASSWORD_HASHER,
      useClass: BcryptPasswordHasher,
    },
    {
      provide: ITOKEN_PROVIDER,
      useClass: JwtTokenProvider,
    },
    {
      provide: APP_FILTER,
      useClass: HttpExceptionFilter,
    },
    {
      provide: 'USER_SERVICE',
      useFactory: (userRepo: any) => {
        return {
          findById: async (id: string) => {
            const user = await userRepo.findById(id);
            if (!user) return null;
            return {
              id: user.getId(),
              email: user.getEmail().getValue(),
              role: user.getRole(),
            };
          }
        };
      },
      inject: [IUSER_REPOSITORY],
    },
  ],
})
export class AppModule {}
