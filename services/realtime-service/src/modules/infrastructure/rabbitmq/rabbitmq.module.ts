import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { RabbitMQModule } from '@delivery/common';
import { RealtimeBroadcastConsumer } from './broadcast.consumer';

@Module({
  imports: [
    RabbitMQModule.registerAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        url: config.get<string>('RABBITMQ_URL') || process.env.RABBITMQ_URL,
        serviceName: 'realtime-service',
      }),
    }),
  ],
  providers: [RealtimeBroadcastConsumer],
  exports: [RealtimeBroadcastConsumer],
})
export class RealtimeRabbitMQModule {}
