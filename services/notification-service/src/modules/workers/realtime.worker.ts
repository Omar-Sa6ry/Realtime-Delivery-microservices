import { Processor, WorkerHost } from '@nestjs/bullmq';
import { Job } from 'bullmq';
import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { DeliveryStateService } from './delivery-state.service';

@Processor('notification-realtime')
@Injectable()
export class RealtimeWorker extends WorkerHost {
  private readonly logger = new Logger(RealtimeWorker.name);

  constructor(
    @InjectRepository(Notification)
    private notificationRepository: Repository<Notification>,
    @InjectRepository(NotificationDelivery)
    private deliveryRepository: Repository<NotificationDelivery>,
    private deliveryState: DeliveryStateService,
  ) {
    super();
  }

  async process(job: Job<{ notificationId: string; deliveryId: string }>) {
    const { notificationId, deliveryId } = job.data;

    const delivery = await this.deliveryRepository.findOne({ where: { id: deliveryId } });
    if (!delivery) return;

    const notification = await this.notificationRepository.findOne({
      where: { id: notificationId },
    });
    if (!notification) return;

    if (notification.expiresAt && notification.expiresAt < new Date()) {
      await this.deliveryState.expire(notification);
      return;
    }

    await this.deliveryState.beginProcessing(delivery);

    try {
      // Delivery is dispatched to clients through the NATS outbox.
      await this.deliveryState.complete(delivery, notification, { delivered: true });

      this.logger.debug(`Realtime dispatched via outbox for notification ${notificationId}`);
    } catch (error) {
      await this.deliveryState.fail(delivery, notification, error);

      this.logger.error(`Realtime failed for notification ${notificationId}: ${error.message}`);
      throw error;
    }
  }
}