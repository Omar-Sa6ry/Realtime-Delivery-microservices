import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { BullModule } from '@nestjs/bullmq';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { NotificationOutbox } from '../../common/database/entities/notification-outbox.entity';
import { NotificationPreference } from '../../common/database/entities/notification-preference.entity';
import { NotificationTemplate } from '../../common/database/entities/notification-template.entity';
import { NotificationService } from './notification.service';
import { NotificationDispatcherService } from './notification-dispatcher.service';
import { TemplateService } from './template/template.service';
import { PreferenceService } from './preference/preference.service';
import { NotificationResolver } from './notification.resolver';

@Module({
  imports: [
    TypeOrmModule.forFeature([
      Notification,
      NotificationDelivery,
      NotificationOutbox,
      NotificationPreference,
      NotificationTemplate,
    ]),
    BullModule.registerQueue(
      { name: 'notification-email' },
      { name: 'notification-sms' },
      { name: 'notification-push' },
      { name: 'notification-inapp' },
      { name: 'notification-realtime' },
      { name: 'notification-scheduled' },
      { name: 'notification-retry' },
    ),
  ],
  providers: [
    NotificationService,
    NotificationDispatcherService,
    TemplateService,
    PreferenceService,
    NotificationResolver,
  ],
  exports: [
    NotificationService,
    NotificationDispatcherService,
    TemplateService,
    PreferenceService,
    BullModule,
  ],
})
export class NotificationModule {}
