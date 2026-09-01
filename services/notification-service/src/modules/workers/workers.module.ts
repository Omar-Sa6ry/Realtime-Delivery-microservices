import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { EmailWorker } from './email.worker';
import { SmsWorker } from './sms.worker';
import { PushWorker } from './push.worker';
import { InAppWorker } from './inapp.worker';
import { RealtimeWorker } from './realtime.worker';
import { ScheduledWorker } from './scheduled.worker';
import { RetryWorker } from './retry.worker';
import { DeliveryStateService } from './delivery-state.service';
import { NotificationModule as BtsNotificationModule } from '@bts-soft/notifications';
import { NotificationModule } from '../notification/notification.module';

@Module({
  imports: [
    TypeOrmModule.forFeature([Notification, NotificationDelivery]),
    BtsNotificationModule,
    NotificationModule,
  ],
  providers: [
    EmailWorker,
    SmsWorker,
    PushWorker,
    InAppWorker,
    RealtimeWorker,
    ScheduledWorker,
    RetryWorker,
    DeliveryStateService,
  ],
})
export class WorkersModule {}