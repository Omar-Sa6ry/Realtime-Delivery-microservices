import { Global, Module } from '@nestjs/common';
import { RedisModule } from '@bts-soft/core';

@Global()
@Module({ imports: [RedisModule], exports: [RedisModule] })
export class DeliveryRedisModule {}
