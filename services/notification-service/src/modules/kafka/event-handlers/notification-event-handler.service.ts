import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import {
  NotificationType,
  NotificationPriority,
  NotificationChannel,
  DeliveryKafkaTopics,
  PaymentKafkaTopics,
  MediaKafkaTopics,
  UserKafkaTopics,
} from '@delivery/common';
import { NotificationService } from '../../notification/notification.service';
import { EventHandlerFactory } from './event-handler.factory';
import { IEventHandler, KafkaEventPayload } from './event-handler.interface';

interface HandlerMapping {
  type: NotificationType;
  priority?: NotificationPriority;
  /**
   * Channels that MUST be delivered regardless of user preferences.
   * Used for security/audit events that must always leave a record.
   */
  requiredChannels?: NotificationChannel[];
}

const EVENT_MAPPING: Record<string, HandlerMapping> = {
  // Delivery domain
  [DeliveryKafkaTopics.DELIVERY_CREATED]: { type: NotificationType.DELIVERY_CREATED },
  [DeliveryKafkaTopics.DRIVER_ASSIGNED]: { type: NotificationType.DRIVER_ASSIGNED, priority: NotificationPriority.HIGH },
  [DeliveryKafkaTopics.DRIVER_ACCEPTED]: { type: NotificationType.DRIVER_ACCEPTED },
  [DeliveryKafkaTopics.DELIVERY_PICKED_UP]: { type: NotificationType.DELIVERY_PICKED_UP },
  [DeliveryKafkaTopics.DELIVERY_IN_TRANSIT]: { type: NotificationType.DELIVERY_IN_TRANSIT },
  [DeliveryKafkaTopics.DELIVERY_COMPLETED]: { type: NotificationType.DELIVERY_COMPLETED },
  [DeliveryKafkaTopics.DELIVERY_CANCELLED]: { type: NotificationType.DELIVERY_CANCELLED },

  // Payment domain
  [PaymentKafkaTopics.PAYMENT_COMPLETED]: { type: NotificationType.PAYMENT_COMPLETED },
  [PaymentKafkaTopics.PAYMENT_FAILED]: { type: NotificationType.PAYMENT_FAILED, priority: NotificationPriority.HIGH },
  [PaymentKafkaTopics.PAYMENT_REFUNDED]: { type: NotificationType.PAYMENT_REFUNDED },

  // Media domain
  [MediaKafkaTopics.UPLOAD_COMPLETED]: { type: NotificationType.MEDIA_UPLOAD_COMPLETED },
  [MediaKafkaTopics.UPLOAD_ABORTED]: { type: NotificationType.MEDIA_UPLOAD_FAILED },
  [MediaKafkaTopics.SCAN_FAILED]: { type: NotificationType.MEDIA_SCAN_FAILED },
  [MediaKafkaTopics.PROCESSING_FAILED]: { type: NotificationType.MEDIA_PROCESSING_FAILED },
  [MediaKafkaTopics.MEDIA_READY]: { type: NotificationType.MEDIA_READY },
  [MediaKafkaTopics.MEDIA_DELETED]: { type: NotificationType.MEDIA_DELETED },
  [MediaKafkaTopics.DELETE_FAILED]: { type: NotificationType.MEDIA_DELETE_FAILED },

  // User domain
  [UserKafkaTopics.USER_CREATED]: { type: NotificationType.USER_REGISTERED },
  [UserKafkaTopics.PASSWORD_RESET_REQUESTED]: {
    type: NotificationType.PASSWORD_RESET_REQUESTED,
    priority: NotificationPriority.HIGH,
    // Security/audit event: always leave an in-app record regardless of preferences
    requiredChannels: [NotificationChannel.IN_APP],
  },
};

function extractUserId(payload: KafkaEventPayload): string | null {
  if (typeof payload.userId === 'string' && payload.userId) return payload.userId;
  if (typeof payload.user_id === 'string' && payload.user_id) return payload.user_id;

  // Events are wrapped in a standard envelope: { eventId, eventType, payload: { ... } }
  const businessPayload = payload.payload && typeof payload.payload === 'object' ? payload.payload : payload.data;
  if (businessPayload && typeof businessPayload === 'object') {
    for (const key of ['userId', 'user_id', 'customerId', 'customer_id', 'driverId', 'driver_id']) {
      const value = businessPayload[key];
      if (typeof value === 'string' && value) return value;
    }
  }
  return null;
}

@Injectable()
export class NotificationEventHandler implements IEventHandler, OnModuleInit {
  private readonly logger = new Logger(NotificationEventHandler.name);

  constructor(
    private readonly notificationService: NotificationService,
    private readonly eventHandlerFactory: EventHandlerFactory,
  ) {}

  onModuleInit() {
    for (const eventType of Object.keys(EVENT_MAPPING)) {
      this.eventHandlerFactory.registerHandler(eventType, this);
    }
    this.logger.log(`Registered ${Object.keys(EVENT_MAPPING).length} event handlers`);
  }

  async handle(payload: KafkaEventPayload): Promise<void> {
    const eventType = payload.eventType;
    if (!eventType) return;

    const mapping = EVENT_MAPPING[eventType];
    if (!mapping) {
      this.logger.debug(`No mapping for event type: ${eventType}`);
      return;
    }

    const userId = extractUserId(payload);
    if (!userId) {
      this.logger.warn(`Skipping event ${eventType}: no target user found`);
      return;
    }

    await this.notificationService.createAndDispatch({
      userId,
      type: mapping.type,
      priority: mapping.priority,
      requiredChannels: mapping.requiredChannels,
      data: {
        ...(payload.payload && typeof payload.payload === 'object' ? payload.payload : {}),
        ...(payload.data && typeof payload.data === 'object' ? payload.data : {}),
        eventId: payload.eventId || payload.id,
      },
    });
  }
}
