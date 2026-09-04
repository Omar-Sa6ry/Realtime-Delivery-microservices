import { Injectable, Logger } from '@nestjs/common';
import { InjectQueue } from '@nestjs/bullmq';
import { InjectRepository } from '@nestjs/typeorm';
import { Queue } from 'bullmq';
import { Repository } from 'typeorm';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationOutbox, OutboxStatus } from '../../common/database/entities/notification-outbox.entity';
import { NotificationChannel, NotificationPriority } from '@delivery/common';

@Injectable()
export class NotificationDispatcherService {
  private readonly logger = new Logger(NotificationDispatcherService.name);

  constructor(
    @InjectQueue('notification-email') private emailQueue: Queue,
    @InjectQueue('notification-sms') private smsQueue: Queue,
    @InjectQueue('notification-push') private pushQueue: Queue,
    @InjectQueue('notification-inapp') private inappQueue: Queue,
    @InjectQueue('notification-realtime') private realtimeQueue: Queue,
    @InjectQueue('notification-scheduled') private scheduledQueue: Queue,
    @InjectRepository(NotificationOutbox)
    private outboxRepository: Repository<NotificationOutbox>,
  ) {}

  async dispatch(notification: Notification, deliveries: NotificationDelivery[]) {
    for (const delivery of deliveries) {
      const jobData = {
        notificationId: notification.id,
        deliveryId: delivery.id,
      };

      const jobId = `${notification.id}-${delivery.channel}`;

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

    if (deliveries.some((d) => d.channel === NotificationChannel.REALTIME)) {
      await this.createRealtimeOutbox(notification);
    }
  }

  async schedule(notification: Notification, scheduledAt: Date) {
    const delay = Math.max(0, scheduledAt.getTime() - Date.now());
    try {
      await this.scheduledQueue.add(
        'dispatch',
        { notificationId: notification.id },
        { delay, jobId: `scheduled-${notification.id}` },
      );
      this.logger.debug(`Scheduled notification ${notification.id} for ${scheduledAt.toISOString()}`);
    } catch (error) {
      this.logger.error(`Failed to schedule notification ${notification.id}: ${error.message}`, error.stack);
    }
  }

  async requeue(delivery: NotificationDelivery) {
    const jobData = {
      notificationId: delivery.notificationId,
      deliveryId: delivery.id,
    };

    const jobId = `retry:${delivery.id}`;

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
      this.logger.debug(`Requeued ${delivery.channel} delivery ${delivery.id} for retry`);
    } catch (error) {
      this.logger.error(`Failed to requeue delivery ${delivery.id}: ${error.message}`, error.stack);
    }
  }

  private async createRealtimeOutbox(notification: Notification) {
    try {
      const existing = await this.outboxRepository.findOne({
        where: { eventType: 'NOTIFICATION_CREATED', aggregateId: notification.id },
      });
      if (existing) return;

      const outbox = this.outboxRepository.create({
        eventType: 'NOTIFICATION_CREATED',
        aggregateId: notification.id,
        payload: {
          userId: notification.userId,
          notificationId: notification.id,
          type: notification.type,
          title: notification.title,
          body: notification.body,
          data: notification.data ?? null,
          priority: notification.priority || NotificationPriority.NORMAL,
        },
      });
      await this.outboxRepository.save(outbox);
    } catch (error) {
      this.logger.error(`Failed to create realtime outbox for notification ${notification.id}: ${error.message}`, error.stack);
    }
  }
}