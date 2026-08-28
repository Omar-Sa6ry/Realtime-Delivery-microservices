import { Global, Module } from '@nestjs/common';
import { RealtimeNatsService } from './nats.service';
import { NatsPublisher } from './nats.publisher';

@Global()
@Module({ providers: [RealtimeNatsService, NatsPublisher], exports: [RealtimeNatsService, NatsPublisher] })
export class NatsModule {}
