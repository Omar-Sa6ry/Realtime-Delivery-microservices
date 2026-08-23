import { Module, Global } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { BullModule } from '@nestjs/bullmq';
import { RedisModule, NotificationModule, RedisService } from '@bts-soft/core';
import { ConfigModule } from '@nestjs/config';
import { User } from './database/entities/user.entity';
import { Address } from './database/entities/address.entity';
import { Outbox } from './database/entities/outbox.entity';
import { RedisSessionRepository } from './database/repositories/redis-session.repository';
import { BcryptPasswordHasher } from './security/bcrypt-password.hasher';
import { JwtTokenProvider } from './security/jwt-token.provider';
import { OutboxWorkerService } from './messaging/outbox-worker.service';
import { OutboxProcessor } from './messaging/outbox.queue';
import { GraphQLResponseInterceptor } from '@delivery/common';

@Global()
@Module({
  imports: [
    ConfigModule,
    RedisModule,
    NotificationModule,
    TypeOrmModule.forFeature([User, Address, Outbox]),
    BullModule.registerQueue({
      name: 'outbox-queue',
    }),
  ],
  providers: [
    BcryptPasswordHasher,
    JwtTokenProvider,
    RedisSessionRepository,
    OutboxWorkerService,
    OutboxProcessor,
    GraphQLResponseInterceptor,
    {
      provide: 'SHARED_REDIS_SERVICE',
      useExisting: RedisService,
    },
  ],
  exports: [
    ConfigModule,
    RedisModule,
    NotificationModule,
    TypeOrmModule,
    BullModule,
    BcryptPasswordHasher,
    JwtTokenProvider,
    RedisSessionRepository,
    OutboxWorkerService,
    OutboxProcessor,
    GraphQLResponseInterceptor,
    'SHARED_REDIS_SERVICE',
  ],
})
export class CommonModule {}
