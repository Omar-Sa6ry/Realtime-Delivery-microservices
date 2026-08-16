import { Processor, WorkerHost } from '@nestjs/bullmq';
import { Job } from 'bullmq';
import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { NotificationService, ChannelType } from '@bts-soft/notifications';
import { DeliveryStateService } from './delivery-state.service';
import { buildChannelMessage } from './channel-message.helper';

@Processor('notification-email')
@Injectable()
export class EmailWorker extends WorkerHost {
  private readonly logger = new Logger(EmailWorker.name);

  constructor(
    @InjectRepository(Notification)
    private notificationRepository: Repository<Notification>,
    @InjectRepository(NotificationDelivery)
    private deliveryRepository: Repository<NotificationDelivery>,
    private deliveryState: DeliveryStateService,
    private notificationService: NotificationService,
  ) {
    super();
  }

  async process(job: Job<{ notificationId: string; deliveryId: string }>) {
    const { notificationId, deliveryId } = job.data;

    const delivery = await this.deliveryRepository.findOne({ where: { id: deliveryId } });
    if (!delivery) return;

    const notification = await this.notificationRepository.findOne({ where: { id: notificationId } });
    if (!notification) return;

    await this.deliveryState.beginProcessing(delivery);

    try {
      await this.notificationService.send(
        ChannelType.EMAIL,
        buildChannelMessage(notification, `email:${notification.id}:${delivery.id}`),
      );

      await this.deliveryState.complete(delivery, notification);

      this.logger.debug(`Email sent successfully for notification ${notificationId}`);
    } catch (error) {
      await this.deliveryState.fail(delivery, notification, error);

      this.logger.error(`Email failed for notification ${notificationId}: ${error.message}`);
      throw error; // Let BullMQ handle retry
    }
  }
}