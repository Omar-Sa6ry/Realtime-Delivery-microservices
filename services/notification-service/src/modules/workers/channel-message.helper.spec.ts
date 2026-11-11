import { buildChannelMessage } from './channel-message.helper';
import { Notification } from '../../common/database/entities/notification.entity';
import {
  NotificationPriority as ShardPriority,
  NotificationType,
} from '@delivery-micro/shard';
import {
  NotificationPriority as BtsPriority,
  NotificationMessage,
} from '@bts-soft/notifications';

function makeNotification(overrides: Partial<Notification> = {}): Notification {
  return {
    id: 'n-1',
    userId: 'user-1',
    type: NotificationType.DELIVERY_CREATED,
    title: 'Order Created',
    body: 'Your order has been created',
    data: { orderId: 'ord-1' },
    priority: ShardPriority.NORMAL,
    ...overrides,
  } as Notification;
}

describe('buildChannelMessage', () => {
  it.each([
    [ShardPriority.LOW, BtsPriority.LOW],
    [ShardPriority.NORMAL, BtsPriority.NORMAL],
    [ShardPriority.HIGH, BtsPriority.HIGH],
    [ShardPriority.CRITICAL, BtsPriority.CRITICAL],
  ])('maps %s priority to %s', (shardPriority, btsPriority) => {
    const message = buildChannelMessage(
      makeNotification({ priority: shardPriority }),
      'idem-1',
    );
    expect(message.priority).toBe(btsPriority);
  });

  it('falls back to NORMAL for unknown priority', () => {
    const message = buildChannelMessage(
      makeNotification({ priority: 'UNKNOWN' as ShardPriority }),
      'idem-1',
    );
    expect(message.priority).toBe(BtsPriority.NORMAL);
  });

  it('passes recipientId, title, body, idempotencyKey and context', () => {
    const notification = makeNotification();
    const message: NotificationMessage = buildChannelMessage(
      notification,
      'email:n-1:d-1',
    );

    expect(message.recipientId).toBe('user-1');
    expect(message.title).toBe('Order Created');
    expect(message.subject).toBe('Order Created');
    expect(message.body).toBe('Your order has been created');
    expect(message.idempotencyKey).toBe('email:n-1:d-1');
    expect(message.context).toEqual({ orderId: 'ord-1' });
  });

  it('sets context to undefined when data is null', () => {
    const message = buildChannelMessage(makeNotification({ data: null }), 'k');
    expect(message.context).toBeUndefined();
  });
});
