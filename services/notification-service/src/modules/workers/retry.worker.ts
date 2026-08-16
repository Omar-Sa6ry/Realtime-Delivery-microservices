import { Processor, WorkerHost } from '@nestjs/bullmq';
import { Queue, Job } from 'bullmq';
import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { InjectQueue } from '@nestjs/bullmq';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, In, LessThan } from 'typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { NotificationStatus, DeliveryChannelStatus } from '@delivery/common';
import { NotificationDispatcherService } from '../notification/notification-dispatcher.service';
import { DeliveryStateService } from './delivery-state.service';

const MAX_ATTEMPTS = 5;
const RETRY_BACKOFF_MS = 60_000;
const SWEEP_INTERVAL_MS = 60_000;

@Processor('notification-retry')
@Injectable()
export class RetryWorker extends WorkerHost implements OnModuleInit {
  private readonly logger = new Logger(RetryWorker.name);

  constructor(
    @InjectQueue('notification-retry') private retryQueue: Queue,
    @InjectRepository(Notification)
    private notificationRepository: Repository<Notification>,
    @InjectRepository(NotificationDelivery)
    private deliveryRepository: Repository<NotificationDelivery>,
    private dispatcherService: NotificationDispatcherService,
    private deliveryState: DeliveryStateService,
  ) {
    super();
  }

  async onModuleInit() {
    await this.retryQueue.upsertJobScheduler(
      'notification-sweep',
      { every: SWEEP_INTERVAL_MS },
      { name: 'sweep', data: {} },
    );
  }

  async process(job: Job) {
    if (job.name === 'sweep') {
      await this.sweep();
    }
  }

  private async sweep() {
    await this.expireDueNotifications();
    await this.requeueFailedDeliveries();
  }

  private async expireDueNotifications() {
    const due = await this.notificationRepository.find({
      where: {
        status: In([
          NotificationStatus.CREATED,
          NotificationStatus.QUEUED,
          NotificationStatus.PROCESSING,
        ]),
        expiresAt: LessThan(new Date()),
      },
    });

    for (const notification of due) {
      await this.deliveryState.expire(notification);
    }

    if (due.length > 0) {
      this.logger.debug(`Expired ${due.length} notification(s)`);
    }
  }

  private async requeueFailedDeliveries() {
    const cutoff = new Date(Date.now() - RETRY_BACKOFF_MS);
    const failed = await this.deliveryRepository.find({
      where: {
        status: DeliveryChannelStatus.FAILED,
        attemptCount: LessThan(MAX_ATTEMPTS),
        failedAt: LessThan(cutoff),
      },
    });

    for (const delivery of failed) {
      delivery.status = DeliveryChannelStatus.PENDING;
      await this.deliveryRepository.save(delivery);
      await this.dispatcherService.requeue(delivery);
    }

    if (failed.length > 0) {
      this.logger.debug(`Requeued ${failed.length} failed delivery(ies)`);
    }
  }
}