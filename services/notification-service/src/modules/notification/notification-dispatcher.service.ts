import { Injectable, Logger } from '@nestjs/common';
import { InjectQueue } from '@nestjs/bullmq';
import { Queue } from 'bullmq';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationChannel } from '@delivery/common';

@Injectable()
export class NotificationDispatcherService {
  private readonly logger = new Logger(NotificationDispatcherService.name);

  constructor(
    @InjectQueue('notification-email') private emailQueue: Queue,
    @InjectQueue('notification-sms') private smsQueue: Queue,
    @InjectQueue('notification-push') private pushQueue: Queue,
    @InjectQueue('notification-inapp') private inappQueue: Queue,
    @InjectQueue('notification-realtime') private realtimeQueue: Queue,
  ) {}

  async dispatch(notification: Notification, deliveries: NotificationDelivery[]) {
    for (const delivery of deliveries) {
      const jobData = {
        notificationId: notification.id,
        deliveryId: delivery.id,
      };

      const jobId = `${notification.id}:${delivery.channel}`;

      try {
        switch (delivery.channel) {
          case NotificationChannel.EMAIL:
            await this.emailQueue.add('send', jobData, { jobId });
            break;
          case NotificationChannel.SMS:
            await this.smsQueue.add('send', jobData, { jobId });
            break;
          case NotificationChannel.PUSH:
            await this.pushQueue.add('send', jobData, { jobId });
            break;
          case NotificationChannel.IN_APP:
            await this.inappQueue.add('send', jobData, { jobId });
            break;
          case NotificationChannel.REALTIME:
            await this.realtimeQueue.add('send', jobData, { jobId });
            break;
        }
        this.logger.debug(`Dispatched ${delivery.channel} job for notification ${notification.id}`);
      } catch (error) {
        this.logger.error(`Failed to dispatch ${delivery.channel} for notification ${notification.id}: ${error.message}`, error.stack);
      }
    }
  }
}
