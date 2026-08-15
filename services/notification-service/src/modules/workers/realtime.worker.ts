import { Processor, WorkerHost } from '@nestjs/bullmq';
import { Job } from 'bullmq';
import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { DeliveryChannelStatus } from '@delivery/common';

@Processor('notification-realtime')
@Injectable()
export class RealtimeWorker extends WorkerHost {
  private readonly logger = new Logger(RealtimeWorker.name);

  constructor(
    @InjectRepository(Notification)
    private notificationRepository: Repository<Notification>,
    @InjectRepository(NotificationDelivery)
    private deliveryRepository: Repository<NotificationDelivery>,
  ) {
    super();
  }

  async process(job: Job<{ notificationId: string; deliveryId: string }>) {
    const { notificationId, deliveryId } = job.data;
    
    const delivery = await this.deliveryRepository.findOne({ where: { id: deliveryId } });
    if (!delivery) return;

    delivery.status = DeliveryChannelStatus.PROCESSING;
    delivery.attemptCount += 1;
    await this.deliveryRepository.save(delivery);

    try {
      const notification = await this.notificationRepository.findOne({ where: { id: notificationId } });
      if (!notification) throw new Error('Notification not found');

      delivery.status = DeliveryChannelStatus.SENT;
      delivery.sentAt = new Date();
      await this.deliveryRepository.save(delivery);
      
      this.logger.debug(`Realtime dispatched via outbox for notification ${notificationId}`);
    } catch (error) {
      delivery.status = DeliveryChannelStatus.FAILED;
      delivery.lastError = error.message;
      delivery.failedAt = new Date();
      await this.deliveryRepository.save(delivery);
      
      this.logger.error(`Realtime failed for notification ${notificationId}: ${error.message}`);
      throw error;
    }
  }
}
