import { Global, Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { RabbitMQModule } from '@delivery/common';
import { UserRabbitMQPublisher } from './rabbitmq.publisher';

@Global()
@Module({
  imports: [
    RabbitMQModule.registerAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        url: config.get<string>('RABBITMQ_URL') || process.env.RABBITMQ_URL,
        serviceName: 'user-service',
      }),
    }),
  ],
  providers: [UserRabbitMQPublisher],
  exports: [UserRabbitMQPublisher],
})
export class UserRabbitMQModule {}
