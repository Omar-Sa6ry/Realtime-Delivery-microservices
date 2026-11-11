import { SmsWorker } from './sms.worker';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { DeliveryStateService } from './delivery-state.service';
import {
  ChannelType,
  NotificationService as BtsNotificationService,
} from '@bts-soft/notifications';
import { NotificationPriority, NotificationType } from '@delivery-micro/shard';
import { createMockRepository, MockRepository } from '../../test-utils';

describe('SmsWorker', () => {
  let worker: SmsWorker;
  let notificationRepository: MockRepository<Notification>;
  let deliveryRepository: MockRepository<NotificationDelivery>;
  let deliveryState: jest.Mocked<DeliveryStateService>;
  let btsNotificationService: { send: jest.Mock };

  const job = () => ({ data: { notificationId: 'n-1', deliveryId: 'd-1' } });

  const notification = {
    id: 'n-1',
    userId: 'user-1',
    type: NotificationType.DELIVERY_CREATED,
    title: 'Title',
    body: 'Body',
    data: null,
    priority: NotificationPriority.NORMAL,
  } as Notification;

  const delivery = { id: 'd-1', notificationId: 'n-1' } as NotificationDelivery;

  beforeEach(() => {
    notificationRepository = createMockRepository<Notification>();
    deliveryRepository = createMockRepository<NotificationDelivery>();
    deliveryState = {
      beginProcessing: jest.fn(),
      complete: jest.fn(),
      fail: jest.fn(),
      expire: jest.fn(),
    } as any;
    btsNotificationService = { send: jest.fn().mockResolvedValue(undefined) };

    worker = new SmsWorker(
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
  });

  it('sends via SMS channel and completes the delivery', async () => {
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue(notification);

    await worker.process(job() as any);

    expect(btsNotificationService.send).toHaveBeenCalledWith(
      ChannelType.SMS,
      expect.objectContaining({ idempotencyKey: 'sms:n-1:d-1' }),
    );
    expect(deliveryState.complete).toHaveBeenCalledWith(delivery, notification);
  });

  it('fails the delivery and rethrows on send errors', async () => {
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue(notification);
    const error = new Error('provider down');
    btsNotificationService.send.mockRejectedValue(error);

    await expect(worker.process(job() as any)).rejects.toThrow('provider down');
    expect(deliveryState.fail).toHaveBeenCalledWith(
      delivery,
      notification,
      error,
    );
  });
});
