import { Global, Module } from '@nestjs/common';
import { RealtimeNatsService } from './nats.service';
import { NatsPublisher } from './nats.publisher';
import { NatsSubscriber } from './nats.subscriber';

@Global()
@Module({
  providers: [RealtimeNatsService, NatsPublisher, NatsSubscriber],
  exports: [RealtimeNatsService, NatsPublisher, NatsSubscriber],
})
export class NatsModule {}