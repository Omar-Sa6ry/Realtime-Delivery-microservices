import { Module, Global } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { RedisModule, RedisService } from '@bts-soft/cache';
import { HealthController } from './health.controller';
import { Notification } from './database/entities/notification.entity';
import { NotificationDelivery } from './database/entities/notification-delivery.entity';
import { NotificationPreference } from './database/entities/notification-preference.entity';
import { NotificationTemplate } from './database/entities/notification-template.entity';
import { NotificationInbox } from './database/entities/notification-inbox.entity';
import { NotificationOutbox } from './database/entities/notification-outbox.entity';

@Global()
@Module({
  controllers: [HealthController],
  imports: [
    RedisModule,
    TypeOrmModule.forFeature([
      Notification,
      NotificationDelivery,
      NotificationPreference,
      NotificationTemplate,
      NotificationInbox,
      NotificationOutbox,
    ]),
  ],
  providers: [
    {
      provide: 'SHARED_REDIS_SERVICE',
      useExisting: RedisService,
    },
  ],
  exports: [
    TypeOrmModule,
    RedisModule,
    'SHARED_REDIS_SERVICE',
  ],
})
export class CommonModule {}