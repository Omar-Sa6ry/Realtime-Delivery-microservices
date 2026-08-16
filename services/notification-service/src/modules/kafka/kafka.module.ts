import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { TypeOrmModule } from '@nestjs/typeorm';
import { KafkaModule } from '@delivery/common';
import { KafkaConsumer } from './kafka.consumer';
import { EventHandlerFactory } from './event-handlers/event-handler.factory';
import { NotificationEventHandler } from './event-handlers/notification-event-handler.service';
import { NotificationModule } from '../notification/notification.module';
import { NotificationInbox } from '../../common/database/entities/notification-inbox.entity';

@Module({
  imports: [
    TypeOrmModule.forFeature([NotificationInbox]),
    NotificationModule,
    KafkaModule.registerAsync({
      imports: [ConfigModule],
      useFactory: (config: ConfigService) => ({
        clientId: 'notification-service',
        brokers: (config.get<string>('KAFKA_BROKERS', 'kafka-srv:9092') || '')
          .split(',').map(b => b.trim()).filter(Boolean),
      }),
      inject: [ConfigService],
    }),
  ],
  providers: [KafkaConsumer, EventHandlerFactory, NotificationEventHandler],
})
export class KafkaConsumerModule {}