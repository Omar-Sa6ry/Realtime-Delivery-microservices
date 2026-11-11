import { DeliveryStateService } from './delivery-state.service';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import {
  DeliveryChannelStatus,
  NotificationStatus,
  NotificationType,
} from '@delivery-micro/shard';
import { createMockRepository, MockRepository } from '../../test-utils';

describe('DeliveryStateService', () => {
  let service: DeliveryStateService;
  let notificationRepository: MockRepository<Notification>;
  let deliveryRepository: MockRepository<NotificationDelivery>;

  const makeNotification = (
    overrides: Partial<Notification> = {},
  ): Notification =>
    ({
      id: 'n-1',
      userId: 'user-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 't',
      body: 'b',
      status: NotificationStatus.CREATED,
      ...overrides,
    }) as Notification;

  const makeDelivery = (
    overrides: Partial<NotificationDelivery> = {},
  ): NotificationDelivery =>
    ({
      id: 'd-1',
      notificationId: 'n-1',
      channel: 'EMAIL' as any,
      status: DeliveryChannelStatus.PENDING,
      attemptCount: 0,
      lastError: null,
      ...overrides,
    }) as NotificationDelivery;

  beforeEach(() => {
    notificationRepository = createMockRepository<Notification>();
    deliveryRepository = createMockRepository<NotificationDelivery>();
    service = new DeliveryStateService(
      notificationRepository as any,
      deliveryRepository as any,
    );
  });

  describe('beginProcessing', () => {
    it('sets PROCESSING status and increments attemptCount', async () => {
      const delivery = makeDelivery({ attemptCount: 1 });
      deliveryRepository.save.mockImplementation(async (d: any) => d);

      await service.beginProcessing(delivery);

      expect(delivery.status).toBe(DeliveryChannelStatus.PROCESSING);
      expect(delivery.attemptCount).toBe(2);
      expect(deliveryRepository.save).toHaveBeenCalledWith(delivery);
    });
  });

  describe('complete', () => {
    it('marks the delivery as SENT and transitions the notification', async () => {
      const delivery = makeDelivery();
      const notification = makeNotification();
      deliveryRepository.find.mockResolvedValue([
        makeDelivery({ status: DeliveryChannelStatus.SENT }),
      ]);
      deliveryRepository.save.mockImplementation(async (d: any) => d);
      notificationRepository.save.mockImplementation(async (n: any) => n);

      await service.complete(delivery, notification);

      expect(delivery.status).toBe(DeliveryChannelStatus.SENT);
      expect(delivery.sentAt).toBeInstanceOf(Date);
      expect(delivery.deliveredAt).toBeUndefined();
      expect(notificationRepository.save).toHaveBeenCalledWith(notification);
      expect(notification.status).toBe(NotificationStatus.SENT);
    });

    it('also stamps deliveredAt when options.delivered is true', async () => {
      const delivery = makeDelivery();
      const notification = makeNotification();
      deliveryRepository.find.mockResolvedValue([
        makeDelivery({ status: DeliveryChannelStatus.DELIVERED as any }),
      ]);

      await service.complete(delivery, notification, { delivered: true });

      expect(delivery.deliveredAt).toEqual(delivery.sentAt);
    });

    it('tolerates a null notification', async () => {
      const delivery = makeDelivery();
      deliveryRepository.find.mockResolvedValue([]);

      await expect(service.complete(delivery, null)).resolves.toBeUndefined();
    });
  });

  describe('fail', () => {
    it('stores the error message and marks the delivery FAILED', async () => {
      const delivery = makeDelivery();
      const notification = makeNotification();
      deliveryRepository.find.mockResolvedValue([
        makeDelivery({ status: DeliveryChannelStatus.FAILED }),
      ]);

      await service.fail(delivery, notification, new Error('smtp down'));

      expect(delivery.status).toBe(DeliveryChannelStatus.FAILED);
      expect(delivery.lastError).toBe('smtp down');
      expect(delivery.failedAt).toBeInstanceOf(Date);
      expect(deliveryRepository.save).toHaveBeenCalledWith(delivery);
      expect(notification.status).toBe(NotificationStatus.FAILED);
    });
  });

  describe('expire', () => {
    it('marks the notification EXPIRED and fails in-flight deliveries', async () => {
      const notification = makeNotification();
      const pending = makeDelivery({ id: 'd-pending' });
      const processing = makeDelivery({
        id: 'd-processing',
        status: DeliveryChannelStatus.PROCESSING,
      });
      const alreadySent = makeDelivery({
        id: 'd-sent',
        status: DeliveryChannelStatus.SENT,
      });
      deliveryRepository.find.mockResolvedValue([
        pending,
        processing,
        alreadySent,
      ]);
      notificationRepository.save.mockImplementation(async (n: any) => n);
      deliveryRepository.save.mockImplementation(async (d: any) => d);

      await service.expire(notification);

      expect(notification.status).toBe(NotificationStatus.EXPIRED);
      expect(pending.status).toBe(DeliveryChannelStatus.FAILED);
      expect(pending.lastError).toBe('Expired');
      expect(processing.status).toBe(DeliveryChannelStatus.FAILED);
      expect(alreadySent.status).toBe(DeliveryChannelStatus.SENT);
      expect(deliveryRepository.save).toHaveBeenCalledTimes(2);
    });
  });

  describe('transitionNotification', () => {
    it('does not modify notifications already in a terminal status', async () => {
      const terminal = [
        NotificationStatus.SENT,
        NotificationStatus.DELIVERED,
        NotificationStatus.FAILED,
        NotificationStatus.EXPIRED,
        NotificationStatus.CANCELLED,
      ];

      for (const status of terminal) {
        notificationRepository.save.mockClear();
        await service.complete(makeDelivery(), makeNotification({ status }));
        expect(notificationRepository.save).not.toHaveBeenCalled();
      }
    });

    it('keeps the notification QUEUED while other deliveries are in flight', async () => {
      const notification = makeNotification();
      deliveryRepository.find.mockResolvedValue([
        makeDelivery({ status: DeliveryChannelStatus.SENT }),
        makeDelivery({
          id: 'd-2',
          status: DeliveryChannelStatus.PENDING,
        }),
      ]);

      await service.complete(makeDelivery(), notification);

      expect(notification.status).toBe(NotificationStatus.QUEUED);
      expect(notificationRepository.save).toHaveBeenCalledWith(notification);
    });

    it('moves CREATED notifications to QUEUED when inflight work remains', async () => {
      const notification = makeNotification({
        status: NotificationStatus.CREATED,
      });
      deliveryRepository.find.mockResolvedValue([
        makeDelivery({ status: DeliveryChannelStatus.PROCESSING }),
      ]);

      await service.complete(makeDelivery(), notification);

      expect(notification.status).toBe(NotificationStatus.QUEUED);
    });

    it('does not re-save when status is already QUEUED and work remains', async () => {
      const notification = makeNotification({
        status: NotificationStatus.QUEUED,
      });
      deliveryRepository.find.mockResolvedValue([
        makeDelivery({ status: DeliveryChannelStatus.RETRYING }),
      ]);

      await service.complete(makeDelivery(), notification);

      expect(notificationRepository.save).not.toHaveBeenCalled();
      expect(notification.status).toBe(NotificationStatus.QUEUED);
    });

    it('fails the notification when any channel failed and nothing is inflight', async () => {
      const notification = makeNotification();
      deliveryRepository.find.mockResolvedValue([
        makeDelivery({ status: DeliveryChannelStatus.FAILED }),
        makeDelivery({ id: 'd-2', status: DeliveryChannelStatus.SENT }),
      ]);

      await service.fail(makeDelivery(), notification, new Error('x'));

      expect(notification.status).toBe(NotificationStatus.FAILED);
    });

    it('returns early when the notification has no id', async () => {
      await expect(
        service.complete(makeDelivery(), { status: null } as any),
      ).resolves.toBeUndefined();
    });

    it('returns early when there are no deliveries rows', async () => {
      const notification = makeNotification();
      deliveryRepository.find.mockResolvedValue([]);

      await service.complete(makeDelivery(), notification);

      expect(notificationRepository.save).not.toHaveBeenCalled();
    });
  });
});
