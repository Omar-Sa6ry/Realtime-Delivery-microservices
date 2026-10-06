import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { RabbitMQModule } from '@delivery-micro/shard';
import { DeliveryRabbitMQPublisher } from './rabbitmq.publisher';

@Module({
  imports: [
    RabbitMQModule.registerAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        url: config.get<string>('RABBITMQ_URL') || process.env.RABBITMQ_URL,
        serviceName: 'delivery-service',
      }),
    }),
  ],
  providers: [DeliveryRabbitMQPublisher],
  exports: [DeliveryRabbitMQPublisher],
})
export class DeliveryRabbitMQModule {}
