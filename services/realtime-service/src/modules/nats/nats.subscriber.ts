import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { RealtimeNatsSubjects, ServerMessageType, MessagePriority } from '@delivery/common';
import { Subscription } from 'nats';
import { SubscriptionStore } from '../subscription/subscription.store';
import { ConnectionService } from '../connection/connection.service';
import { SocketWriter } from '../connection/socket-writer.service';
import { RealtimeNatsService } from './nats.service';
import { Role } from '@delivery/common';
import { RealtimeMetricsService } from '../../common/metrics/realtime-metrics.service';

interface NatsFanoutMessage {
  type: ServerMessageType;
  priority?: MessagePriority;
  data: Record<string, unknown>;
}

/**
 * NATS observer: subscribes to all realtime fan-out subjects and pushes to local sockets.
 *  realtime.delivery.location.updated   -> delivery subscribers (LOSSY, coalesced via SocketWriter)
 *  realtime.delivery.status.updated     -> delivery subscribers (NORMAL)
 *  realtime.driver.assignment.updated   -> delivery subscribers (CRITICAL)
 *  realtime.driver.presence.updated     -> admin sockets
 */
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
    const subjects = [
      RealtimeNatsSubjects.DELIVERY_LOCATION_UPDATED,
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
      RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED,
      RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED,
    ];

    for (const subject of subjects) {
      const client = this.nats.getClient();
      if (!client) return;
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

    if (subject === RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED) {
      await this.fanoutToAdmins(message);
    } else {
      await this.fanoutToDeliverySubscribers(message);
    }
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