import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { TypeOrmModule } from '@nestjs/typeorm';
import { RabbitMQModule } from '@delivery/common';
import { NotificationInbox } from '../../common/database/entities/notification-inbox.entity';
import { KafkaConsumerModule } from '../kafka/kafka.module';
import { EmailRabbitMQConsumer } from './consumers/email.consumer';
import { SmsRabbitMQConsumer } from './consumers/sms.consumer';
import { PushRabbitMQConsumer } from './consumers/push.consumer';
import { RabbitMQNotificationMapper } from './rabbitmq-notification.mapper';

@Module({
  imports: [
    TypeOrmModule.forFeature([NotificationInbox]),
    KafkaConsumerModule,
    RabbitMQModule.registerAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        url: config.get<string>('RABBITMQ_URL') || process.env.RABBITMQ_URL,
        serviceName: 'notification-service',
      }),
    }),
  ],
  providers: [
    EmailRabbitMQConsumer,
    SmsRabbitMQConsumer,
    PushRabbitMQConsumer,
    RabbitMQNotificationMapper,
  ],
  exports: [
    EmailRabbitMQConsumer,
    SmsRabbitMQConsumer,
    PushRabbitMQConsumer,
    RabbitMQNotificationMapper,
  ],
})
export class RabbitMQNotificationModule {}
