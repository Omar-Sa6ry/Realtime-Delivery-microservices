import { Processor, WorkerHost } from '@nestjs/bullmq';
import { Job } from 'bullmq';
import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, In } from 'typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { DeliveryChannelStatus } from '@delivery/common';
import { NotificationDispatcherService } from '../notification/notification-dispatcher.service';

@Processor('notification-scheduled')
@Injectable()
export class ScheduledWorker extends WorkerHost {
  private readonly logger = new Logger(ScheduledWorker.name);

  constructor(
    @InjectRepository(Notification)
    private notificationRepository: Repository<Notification>,
    @InjectRepository(NotificationDelivery)
    private deliveryRepository: Repository<NotificationDelivery>,
    private dispatcherService: NotificationDispatcherService,
  ) {
    super();
  }

  async process(job: Job<{ notificationId: string }>) {
    const { notificationId } = job.data;

    const notification = await this.notificationRepository.findOne({ where: { id: notificationId } });
    if (!notification) return;

    const deliveries = await this.deliveryRepository.find({
      where: { notificationId, status: In([DeliveryChannelStatus.PENDING]) },
    });
    if (deliveries.length === 0) return;

    await this.dispatcherService.dispatch(notification, deliveries);
    this.logger.debug(`Dispatched scheduled notification ${notificationId}`);
  }
}