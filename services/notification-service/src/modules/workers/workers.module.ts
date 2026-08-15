import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { EmailWorker } from './email.worker';
import { SmsWorker } from './sms.worker';
import { PushWorker } from './push.worker';
import { InAppWorker } from './inapp.worker';
import { RealtimeWorker } from './realtime.worker';

@Module({
  imports: [
    TypeOrmModule.forFeature([Notification, NotificationDelivery]),
  ],
  providers: [
    EmailWorker,
    SmsWorker,
    PushWorker,
    InAppWorker,
    RealtimeWorker,
  ],
})
export class WorkersModule {}
