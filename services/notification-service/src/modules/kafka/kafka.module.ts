import { Module } from '@nestjs/common';
import { KafkaConsumer } from './kafka.consumer';
import { EventHandlerFactory } from './event-handlers/event-handler.factory';
import { NotificationModule } from '../notification/notification.module';
import { TypeOrmModule } from '@nestjs/typeorm';
import { NotificationInbox } from '../../common/database/entities/notification-inbox.entity';

@Module({
  imports: [
    TypeOrmModule.forFeature([NotificationInbox]),
    NotificationModule,
  ],
  providers: [KafkaConsumer, EventHandlerFactory],
})
export class KafkaConsumerModule {}