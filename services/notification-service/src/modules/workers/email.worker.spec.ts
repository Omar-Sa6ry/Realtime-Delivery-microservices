import { EmailWorker } from './email.worker';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { DeliveryStateService } from './delivery-state.service';
import {
  ChannelType,
  NotificationService as BtsNotificationService,
} from '@bts-soft/notifications';
import { NotificationPriority, NotificationType } from '@delivery-micro/shard';
import { createMockRepository, MockRepository } from '../../test-utils';

describe('EmailWorker', () => {
  let worker: EmailWorker;
  let notificationRepository: MockRepository<Notification>;
  let deliveryRepository: MockRepository<NotificationDelivery>;
  let deliveryState: jest.Mocked<DeliveryStateService>;
  let btsNotificationService: { send: jest.Mock };

  const job = (overrides: any = {}) => ({
    data: { notificationId: 'n-1', deliveryId: 'd-1', ...overrides },
  });

  const notification = {
    id: 'n-1',
    userId: 'user-1',
    type: NotificationType.DELIVERY_CREATED,
    title: 'Title',
    body: 'Body',
    data: { a: 1 },
    priority: NotificationPriority.HIGH,
  } as Notification;

  const delivery = { id: 'd-1', notificationId: 'n-1' } as NotificationDelivery;

  beforeEach(() => {
    notificationRepository = createMockRepository<Notification>();
    deliveryRepository = createMockRepository<NotificationDelivery>();
    deliveryState = {
      beginProcessing: jest.fn().mockResolvedValue(undefined),
      complete: jest.fn().mockResolvedValue(undefined),
      fail: jest.fn().mockResolvedValue(undefined),
      expire: jest.fn().mockResolvedValue(undefined),
    } as any;
    btsNotificationService = { send: jest.fn().mockResolvedValue(undefined) };

    worker = new EmailWorker(
      notificationRepository as any,
      deliveryRepository as any,
      deliveryState,
      btsNotificationService as unknown as BtsNotificationService,
    );
  });

  it('returns quietly when the delivery is missing', async () => {
    deliveryRepository.findOne.mockResolvedValue(null);

    await expect(worker.process(job() as any)).resolves.toBeUndefined();
    expect(btsNotificationService.send).not.toHaveBeenCalled();
  });

  it('returns quietly when the notification is missing', async () => {
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue(null);

    await expect(worker.process(job() as any)).resolves.toBeUndefined();
    expect(deliveryState.beginProcessing).not.toHaveBeenCalled();
  });

  it('runs the happy path: beginProcessing -> send -> complete', async () => {
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue(notification);

    await worker.process(job() as any);

    expect(deliveryState.beginProcessing).toHaveBeenCalledWith(delivery);
    expect(btsNotificationService.send).toHaveBeenCalledWith(
      ChannelType.EMAIL,
      expect.objectContaining({
        recipientId: 'user-1',
        title: 'Title',
        body: 'Body',
        idempotencyKey: 'email:n-1:d-1',
      }),
    );
    expect(deliveryState.complete).toHaveBeenCalledWith(delivery, notification);
  });

  it('marks failure and rethrows so BullMQ can retry', async () => {
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue(notification);
    const sendError = new Error('smtp down');
    btsNotificationService.send.mockRejectedValue(sendError);

    await expect(worker.process(job() as any)).rejects.toThrow('smtp down');
    expect(deliveryState.fail).toHaveBeenCalledWith(
      delivery,
      notification,
      sendError,
    );
  });
});
