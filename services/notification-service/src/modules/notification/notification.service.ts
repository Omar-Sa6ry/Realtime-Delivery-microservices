import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, DataSource } from 'typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { NotificationOutbox } from '../../common/database/entities/notification-outbox.entity';
import { NotificationDispatcherService } from './notification-dispatcher.service';
import { NotificationType, NotificationPriority, NotificationChannel } from '@delivery/common';
import { TemplateService } from './template/template.service';
import { PreferenceService } from './preference/preference.service';

export interface CreateNotificationParams {
  userId: string;
  type: NotificationType;
  title?: string;
  body?: string;
  data?: Record<string, any>;
  priority?: NotificationPriority;
}

@Injectable()
export class NotificationService {
  private readonly logger = new Logger(NotificationService.name);

  constructor(
    @InjectRepository(Notification)
    private notificationRepository: Repository<Notification>,
    private dispatcherService: NotificationDispatcherService,
    private templateService: TemplateService,
    private preferenceService: PreferenceService,
    private dataSource: DataSource,
  ) {}

  async createAndDispatch(params: CreateNotificationParams): Promise<string | null> {
    const { userId, type, data, priority } = params;
    
    // 1. Resolve Preferences
    const channels = await this.preferenceService.getEnabledChannels(userId, type);
    if (channels.length === 0) {
      this.logger.debug(`No channels enabled for user ${userId} for event ${type}`);
      return null;
    }

    // 2. Render Template (assuming English for now, can be extended)
    const locale = 'en';
    const { title, body } = await this.templateService.render(type, channels[0], locale, data || {});

    const notificationTitle = params.title || title;
    const notificationBody = params.body || body;

    // 3. Database Transaction
    const queryRunner = this.dataSource.createQueryRunner();
    await queryRunner.connect();
    await queryRunner.startTransaction();

    try {
      // Create Notification
      const notification = queryRunner.manager.create(Notification, {
        userId,
        type,
        title: notificationTitle,
        body: notificationBody,
        data,
        priority: priority || NotificationPriority.NORMAL,
      });
      await queryRunner.manager.save(notification);

      // Create Deliveries
      const deliveries: NotificationDelivery[] = [];
      for (const channel of channels) {
        const delivery = queryRunner.manager.create(NotificationDelivery, {
          notificationId: notification.id,
          channel,
        });
        deliveries.push(delivery);
      }
      await queryRunner.manager.save(deliveries);

      // Create Outbox for Realtime (if Realtime is enabled)
      if (channels.includes(NotificationChannel.REALTIME)) {
        const outbox = queryRunner.manager.create(NotificationOutbox, {
          eventType: 'NOTIFICATION_CREATED',
          aggregateId: notification.id,
          payload: {
            userId,
            notificationId: notification.id,
            type,
            title: notificationTitle,
            body: notificationBody,
            data,
            priority: priority || NotificationPriority.NORMAL,
          },
        });
        await queryRunner.manager.save(outbox);
      }

      await queryRunner.commitTransaction();
      this.logger.debug(`Saved notification ${notification.id} for user ${userId}`);

      // 4. Dispatch Jobs
      await this.dispatcherService.dispatch(notification, deliveries);

      return notification.id;
    } catch (error) {
      await queryRunner.rollbackTransaction();
      this.logger.error(`Error creating notification: ${error.message}`, error.stack);
      return null;
    } finally {
      await queryRunner.release();
    }
  }
}
