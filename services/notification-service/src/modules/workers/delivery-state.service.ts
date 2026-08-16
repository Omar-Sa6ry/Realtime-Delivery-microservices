import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { NotificationStatus, DeliveryChannelStatus } from '@delivery/common';

@Injectable()
export class DeliveryStateService {
  private readonly logger = new Logger(DeliveryStateService.name);

  constructor(
    @InjectRepository(Notification)
    private notificationRepository: Repository<Notification>,
    @InjectRepository(NotificationDelivery)
    private deliveryRepository: Repository<NotificationDelivery>,
  ) {}

  async beginProcessing(delivery: NotificationDelivery) {
    delivery.status = DeliveryChannelStatus.PROCESSING;
    delivery.attemptCount += 1;
    await this.deliveryRepository.save(delivery);
  }

  async complete(
    delivery: NotificationDelivery,
    notification: Notification | null,
    options: { delivered?: boolean } = {},
  ) {
    delivery.status = DeliveryChannelStatus.SENT;
    delivery.sentAt = new Date();
    if (options.delivered) {
      delivery.deliveredAt = delivery.sentAt;
    }
    await this.deliveryRepository.save(delivery);
    await this.transitionNotification(notification, NotificationStatus.SENT);
  }

  async fail(delivery: NotificationDelivery, notification: Notification | null, error: Error) {
    delivery.status = DeliveryChannelStatus.FAILED;
    delivery.lastError = error.message;
    delivery.failedAt = new Date();
    await this.deliveryRepository.save(delivery);
    await this.transitionNotification(notification, NotificationStatus.FAILED);
  }

  async expire(notification: Notification) {
    notification.status = NotificationStatus.EXPIRED;
    await this.notificationRepository.save(notification);

    const deliveries = await this.deliveryRepository.find({
      where: { notificationId: notification.id },
    });
    for (const delivery of deliveries) {
      if (
        delivery.status === DeliveryChannelStatus.PENDING ||
        delivery.status === DeliveryChannelStatus.PROCESSING ||
        delivery.status === DeliveryChannelStatus.RETRYING
      ) {
        delivery.status = DeliveryChannelStatus.FAILED;
        delivery.lastError = 'Expired';
        delivery.failedAt = new Date();
        await this.deliveryRepository.save(delivery);
      }
    }
    this.logger.debug(`Expired notification ${notification.id}`);
  }

  private async transitionNotification(notification: Notification | null, terminalStatus: NotificationStatus) {
    if (!notification || notification.id === undefined) return;
    if (
      notification.status === NotificationStatus.SENT ||
      notification.status === NotificationStatus.DELIVERED ||
      notification.status === NotificationStatus.FAILED ||
      notification.status === NotificationStatus.EXPIRED ||
      notification.status === NotificationStatus.CANCELLED
    ) {
      return;
    }

    const deliveries = await this.deliveryRepository.find({
      where: { notificationId: notification.id },
    });
    if (deliveries.length === 0) return;

    const hasInflight = deliveries.some(
      (d) =>
        d.status === DeliveryChannelStatus.PENDING ||
        d.status === DeliveryChannelStatus.PROCESSING ||
        d.status === DeliveryChannelStatus.QUEUED ||
        d.status === DeliveryChannelStatus.RETRYING,
    );

    if (hasInflight) {
      if (notification.status === NotificationStatus.CREATED) {
        notification.status = NotificationStatus.QUEUED;
        await this.notificationRepository.save(notification);
      }
      return;
    }

    const hasFailed = deliveries.some((d) => d.status === DeliveryChannelStatus.FAILED);
    notification.status = hasFailed ? NotificationStatus.FAILED : terminalStatus;
    await this.notificationRepository.save(notification);
  }
}