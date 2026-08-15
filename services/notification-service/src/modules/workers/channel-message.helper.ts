import {
  NotificationMessage,
  NotificationPriority as BtsNotificationPriority,
} from '@bts-soft/notifications';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationPriority } from '@delivery/common';

const NOTIFICATION_PRIORITY_MAP: Record<NotificationPriority, BtsNotificationPriority> = {
  [NotificationPriority.LOW]: BtsNotificationPriority.LOW,
  [NotificationPriority.NORMAL]: BtsNotificationPriority.NORMAL,
  [NotificationPriority.HIGH]: BtsNotificationPriority.HIGH,
  [NotificationPriority.CRITICAL]: BtsNotificationPriority.CRITICAL,
};

export function buildChannelMessage(
  notification: Notification,
  idempotencyKey: string,
): NotificationMessage {
  return {
    recipientId: notification.userId,
    body: notification.body,
    title: notification.title,
    subject: notification.title,
    priority:
      NOTIFICATION_PRIORITY_MAP[notification.priority] ?? BtsNotificationPriority.NORMAL,
    idempotencyKey,
    context: notification.data ?? undefined,
  };
}