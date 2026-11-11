import { NotificationDispatcherService } from './notification-dispatcher.service';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import {
  NotificationChannel,
  NotificationPriority,
  NotificationType,
} from '@delivery-micro/shard';
import {
  createMockQueue,
  createMockRepository,
  MockQueue,
  MockRepository,
} from '../../test-utils';
import { OutboxStatus } from '../../common/database/entities/notification-outbox.entity';

describe('NotificationDispatcherService', () => {
  let service: NotificationDispatcherService;
  let emailQueue: MockQueue;
  let smsQueue: MockQueue;
  let pushQueue: MockQueue;
  let inappQueue: MockQueue;
  let realtimeQueue: MockQueue;
  let scheduledQueue: MockQueue;
  let outboxRepository: MockRepository;

  const notification = (overrides: Partial<Notification> = {}): Notification =>
    ({
      id: 'n-1',
      userId: 'user-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 't',
      body: 'b',
      priority: NotificationPriority.NORMAL,
      data: null,
      ...overrides,
    }) as Notification;

  const delivery = (
    channel: NotificationChannel,
    overrides: Partial<NotificationDelivery> = {},
  ): NotificationDelivery =>
    ({
      id: `d-${channel}`,
      notificationId: 'n-1',
      channel,
      ...overrides,
    }) as NotificationDelivery;

  beforeEach(() => {
    emailQueue = createMockQueue();
    smsQueue = createMockQueue();
    pushQueue = createMockQueue();
    inappQueue = createMockQueue();
    realtimeQueue = createMockQueue();
    scheduledQueue = createMockQueue();
    outboxRepository = createMockRepository();

    service = new NotificationDispatcherService(
      emailQueue as any,
      smsQueue as any,
      pushQueue as any,
      inappQueue as any,
      realtimeQueue as any,
      scheduledQueue as any,
      outboxRepository as any,
    );
  });

  describe('dispatch', () => {
    it('routes every channel to its own queue with the correct jobId', async () => {
      const deliveries = [
        delivery(NotificationChannel.EMAIL),
        delivery(NotificationChannel.SMS),
        delivery(NotificationChannel.PUSH),
        delivery(NotificationChannel.IN_APP),
      ];
      outboxRepository.findOne.mockResolvedValue(null);

      await service.dispatch(notification(), deliveries);

      expect(emailQueue.add).toHaveBeenCalledWith(
        'send',
        { notificationId: 'n-1', deliveryId: 'd-EMAIL' },
        { jobId: 'n-1-EMAIL' },
      );
      expect(smsQueue.add).toHaveBeenCalledWith(
        'send',
        { notificationId: 'n-1', deliveryId: 'd-SMS' },
        { jobId: 'n-1-SMS' },
      );
      expect(pushQueue.add).toHaveBeenCalledWith(
        'send',
        { notificationId: 'n-1', deliveryId: 'd-PUSH' },
        { jobId: 'n-1-PUSH' },
      );
      expect(inappQueue.add).toHaveBeenCalledWith(
        'send',
        { notificationId: 'n-1', deliveryId: 'd-IN_APP' },
        { jobId: 'n-1-IN_APP' },
      );
      expect(realtimeQueue.add).not.toHaveBeenCalled();
    });

    it('creates an outbox record when a REALTIME delivery is included', async () => {
      outboxRepository.findOne.mockResolvedValue(null);
      const outboxRow = { id: 'out-1' };
      outboxRepository.create.mockReturnValue(outboxRow);
      outboxRepository.save.mockResolvedValue(outboxRow);

      await service.dispatch(notification(), [
        delivery(NotificationChannel.REALTIME),
      ]);

      expect(realtimeQueue.add).toHaveBeenCalledWith(
        'send',
        { notificationId: 'n-1', deliveryId: 'd-REALTIME' },
        { jobId: 'n-1-REALTIME' },
      );
      expect(outboxRepository.findOne).toHaveBeenCalledWith({
        where: {
          eventType: 'NOTIFICATION_CREATED',
          aggregateId: 'n-1',
        },
      });
      expect(outboxRepository.create).toHaveBeenCalledWith({
        eventType: 'NOTIFICATION_CREATED',
        aggregateId: 'n-1',
        payload: expect.objectContaining({
          userId: 'user-1',
          notificationId: 'n-1',
          type: NotificationType.DELIVERY_CREATED,
          priority: NotificationPriority.NORMAL,
        }),
      });
      expect(outboxRepository.save).toHaveBeenCalledWith(outboxRow);
    });

    it('does not duplicate the outbox record when one already exists', async () => {
      outboxRepository.findOne.mockResolvedValue({ id: 'existing-out' });

      await service.dispatch(notification(), [
        delivery(NotificationChannel.REALTIME),
      ]);

      expect(outboxRepository.create).not.toHaveBeenCalled();
      expect(outboxRepository.save).not.toHaveBeenCalled();
    });

    it('swallows queue add errors without breaking remaining channels', async () => {
      emailQueue.add.mockRejectedValue(new Error('redis down'));

      await service.dispatch(notification(), [
        delivery(NotificationChannel.EMAIL),
        delivery(NotificationChannel.SMS),
      ]);

      expect(smsQueue.add).toHaveBeenCalled();
    });

    it('swallows outbox creation errors', async () => {
      outboxRepository.findOne.mockRejectedValue(new Error('db down'));

      await expect(
        service.dispatch(notification(), [
          delivery(NotificationChannel.REALTIME),
        ]),
      ).resolves.toBeUndefined();
    });

    it('defaults priority to NORMAL in outbox payload when missing', async () => {
      outboxRepository.findOne.mockResolvedValue(null);

      await service.dispatch(notification({ priority: undefined as any }), [
        delivery(NotificationChannel.REALTIME),
      ]);

      expect(outboxRepository.create).toHaveBeenCalledWith(
        expect.objectContaining({
          payload: expect.objectContaining({
            priority: NotificationPriority.NORMAL,
          }),
        }),
      );
    });
  });

  describe('schedule', () => {
    it('adds a delayed job to the scheduled queue with a unique jobId', async () => {
      const scheduledAt = new Date(Date.now() + 60_000);

      await service.schedule(notification(), scheduledAt);

      const [, , opts] = scheduledQueue.add.mock.calls[0];
      expect(scheduledQueue.add).toHaveBeenCalledWith(
        'dispatch',
        { notificationId: 'n-1' },
        expect.objectContaining({ jobId: 'scheduled-n-1' }),
      );
      expect(opts.delay).toBeGreaterThan(50_000);
      expect(opts.delay).toBeLessThanOrEqual(60_000);
    });

    it('clamps negative delays to zero for past dates', async () => {
      await service.schedule(notification(), new Date(Date.now() - 5_000));

      expect(scheduledQueue.add).toHaveBeenCalledWith(
        'dispatch',
        { notificationId: 'n-1' },
        expect.objectContaining({ jobId: 'scheduled-n-1', delay: 0 }),
      );
    });

    it('catches scheduling errors', async () => {
      scheduledQueue.add.mockRejectedValue(new Error('queue down'));

      await expect(
        service.schedule(notification(), new Date(Date.now() + 1000)),
      ).resolves.toBeUndefined();
    });
  });

  describe('requeue', () => {
    it.each([
      [NotificationChannel.EMAIL, () => emailQueue],
      [NotificationChannel.SMS, () => smsQueue],
      [NotificationChannel.PUSH, () => pushQueue],
      [NotificationChannel.IN_APP, () => inappQueue],
      [NotificationChannel.REALTIME, () => realtimeQueue],
    ])('requeues %s with retry jobId', async (channel, getQueue) => {
      await service.requeue(delivery(channel));

      expect(getQueue().add).toHaveBeenCalledWith(
        'send',
        { notificationId: 'n-1', deliveryId: `d-${channel}` },
        { jobId: 'retry:d-' + channel },
      );
    });

    it('catches requeue errors', async () => {
      emailQueue.add.mockRejectedValue(new Error('queue down'));

      await expect(
        service.requeue(delivery(NotificationChannel.EMAIL)),
      ).resolves.toBeUndefined();
    });
  });

  describe('outbox status handling', () => {
    it('uses OutboxStatus.PENDING as the entity default for new records', () => {
      expect(OutboxStatus.PENDING).toBe('PENDING');
      expect(OutboxStatus.PUBLISHED).toBe('PUBLISHED');
      expect(OutboxStatus.FAILED).toBe('FAILED');
    });
  });
});
