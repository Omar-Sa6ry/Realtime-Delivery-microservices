import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { RealtimeNatsSubjects, ServerMessageType, MessagePriority, NotificationNatsSubjects } from '@delivery/common';
import { Subscription } from 'nats';
import { SubscriptionStore } from '../../features/subscription/subscription.store';
import { ConnectionService } from '../../gateway/connection/connection.service';
import { SocketWriter } from '../../gateway/connection/socket-writer.service';
import { RealtimeNatsService } from './nats.service';
import { Role } from '@delivery/common';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';

interface NatsFanoutMessage {
  type: ServerMessageType;
  priority?: MessagePriority;
  data: Record<string, unknown>;
}

@Injectable()
export class NatsSubscriber implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(NatsSubscriber.name);

  constructor(
    private readonly nats: RealtimeNatsService,
    private readonly subscriptionStore: SubscriptionStore,
    private readonly connectionService: ConnectionService,
    private readonly writer: SocketWriter,
    private readonly metrics: RealtimeMetricsService,
  ) {}

  async onModuleInit(): Promise<void> {
    this.subscribe();
  }

  private subscribe(): void {
    const client = this.nats.getClient();
    if (!client) {
      setTimeout(() => this.subscribe(), 2000);
      return;
    }

    const subjects = [
      RealtimeNatsSubjects.DELIVERY_LOCATION_UPDATED,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED,
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED,
      RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED,
      'payment.status.updated',
      `${NotificationNatsSubjects.NOTIFICATION_USER}.*`,
      // Media realtime subjects
      RealtimeNatsSubjects.MEDIA_UPLOAD_PROGRESS,
      RealtimeNatsSubjects.MEDIA_PROCESSING_PROGRESS,
      RealtimeNatsSubjects.MEDIA_READY,
      RealtimeNatsSubjects.MEDIA_DELETED,
      RealtimeNatsSubjects.MEDIA_FAILED,
    ];

    for (const subject of subjects) {
      const sub: Subscription = client.subscribe(subject);
      this.consume(subject, sub);
    }
    this.logger.log(`Subscribed to NATS subjects: ${subjects.join(', ')}`);
  }

  private async consume(subject: string, sub: Subscription): Promise<void> {
    try {
      for await (const msg of sub) {
        try {
          await this.handleMessage(subject, msg.data);
        } catch (err) {
          this.logger.error(`NATS handler error on ${subject}: ${err.message}`);
        }
      }
    } catch (err) {
      this.logger.error(`NATS subscription ${subject} terminated: ${err.message}`);
    }
  }

  private async handleMessage(subject: string, data: Uint8Array): Promise<void> {
    let message: NatsFanoutMessage;
    try {
      message = this.nats.getCodec().decode(data) as NatsFanoutMessage;
    } catch (err) {
      this.logger.warn(`Failed to decode NATS message on ${subject}`);
      return;
    }

    if (subject === RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED) {
      const driverId = String(message.data?.driverId || '');
      if (driverId) {
        await this.fanoutToDriver(driverId, message);
      }
    } else if (subject.startsWith(`${NotificationNatsSubjects.NOTIFICATION_USER}.`)) {
      const userId = subject.split('.').pop();
      if (userId) {
        await this.fanoutToUser(userId, message);
      }
    } else if (subject === 'payment.status.updated') {
      const payloadData = (message as any)?.data || (message as any);
      const userId = String(payloadData?.userId || '');
      if (userId) {
        await this.fanoutPaymentStatusToUser(userId, payloadData);
      }
    } else if (subject === RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED) {
      await this.fanoutToAdmins(message);
    } else if (this.isMediaSubject(subject)) {
      await this.fanoutMediaToUser(message);
    } else {
      await this.fanoutToDeliverySubscribers(message);
    }
  }

  private isMediaSubject(subject: string): boolean {
    return subject.startsWith('realtime.media.');
  }

  private async fanoutToDriver(driverId: string, message: NatsFanoutMessage): Promise<void> {
    const sockets = this.connectionService.getLocalSocketsByUser(driverId);
    if (sockets.length === 0) return;

    this.writer.sendMany(
      sockets,
      {
        type: message.type || ServerMessageType.ASSIGNMENT_OFFERED,
        data: message.data,
      },
      message.priority || MessagePriority.CRITICAL,
    );
  }

  private async fanoutToUser(userId: string, message: NatsFanoutMessage): Promise<void> {
    const sockets = this.connectionService.getLocalSocketsByUser(userId);
    if (sockets.length === 0) return;

    this.writer.sendMany(
      sockets,
      {
        type: ServerMessageType.NOTIFICATION_RECEIVED,
        data: message.data,
      },
      message.priority || MessagePriority.NORMAL,
    );
  }

  private async fanoutPaymentStatusToUser(userId: string, data: Record<string, unknown>): Promise<void> {
    const sockets = this.connectionService.getLocalSocketsByUser(userId);
    if (sockets.length === 0) return;

    this.writer.sendMany(
      sockets,
      {
        type: ServerMessageType.PAYMENT_STATUS_CHANGED,
        data,
      },
      MessagePriority.NORMAL,
    );
  }

  private async fanoutMediaToUser(message: NatsFanoutMessage): Promise<void> {
    const userId = String(message.data?.userId || '');
    if (!userId) return;

    const sockets = this.connectionService.getLocalSocketsByUser(userId);
    if (sockets.length === 0) return;

    this.writer.sendMany(
      sockets,
      {
        type: message.type,
        data: message.data,
      },
      message.priority || MessagePriority.NORMAL,
    );
  }

  private async fanoutToDeliverySubscribers(message: NatsFanoutMessage): Promise<void> {
    const deliveryId = String(message.data?.deliveryId || '');
    if (!deliveryId) return;

    const socketIds = await this.subscriptionStore.getDeliverySubscribers(deliveryId);
    const sockets = socketIds
      .map((id) => this.connectionService.getLocalConnection(id))
      .filter((s) => s !== undefined);

    const priority =
      message.priority ||
      (message.type === ServerMessageType.DELIVERY_LOCATION_UPDATED
        ? MessagePriority.HIGH_FREQUENCY_LOSSY
        : MessagePriority.NORMAL);

    this.writer.sendMany(
      sockets,
      {
        type: message.type,
        data: message.data,
      },
      priority,
    );
  }

  private async fanoutToAdmins(message: NatsFanoutMessage): Promise<void> {
    const admins = this.connectionService.getLocalSocketsByRole(Role.ADMIN);
    this.writer.sendMany(admins, {
      type: message.type,
      data: message.data,
    });
  }

  async onModuleDestroy(): Promise<void> {
    await this.nats.onModuleDestroy();
  }
}