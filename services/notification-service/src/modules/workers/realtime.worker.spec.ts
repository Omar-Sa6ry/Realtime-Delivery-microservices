import { RealtimeWorker } from './realtime.worker';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { DeliveryStateService } from './delivery-state.service';
import { NotificationType } from '@delivery-micro/shard';
import { createMockRepository, MockRepository } from '../../test-utils';

describe('RealtimeWorker', () => {
  let worker: RealtimeWorker;
  let notificationRepository: MockRepository<Notification>;
  let deliveryRepository: MockRepository<NotificationDelivery>;
  let deliveryState: jest.Mocked<DeliveryStateService>;

  const job = () => ({ data: { notificationId: 'n-1', deliveryId: 'd-1' } });

  const notification = {
    id: 'n-1',
    userId: 'user-1',
    type: NotificationType.DELIVERY_CREATED,
    title: 'Title',
    body: 'Body',
    expiresAt: null,
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

    worker = new RealtimeWorker(
      notificationRepository as any,
      deliveryRepository as any,
      deliveryState,
    );
  });

  it('returns quietly when the delivery is missing', async () => {
    deliveryRepository.findOne.mockResolvedValue(null);
    await expect(worker.process(job() as any)).resolves.toBeUndefined();
  });

  it('returns quietly when the notification is missing', async () => {
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue(null);
    await expect(worker.process(job() as any)).resolves.toBeUndefined();
  });

  it('expires the notification when expiresAt is in the past', async () => {
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue({
      ...notification,
      expiresAt: new Date(Date.now() - 1000),
    });

    await worker.process(job() as any);

    expect(deliveryState.expire).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'n-1' }),
    );
    expect(deliveryState.beginProcessing).not.toHaveBeenCalled();
  });

  it('dispatches via outbox by completing with delivered: true when not expired', async () => {
    const futureNotification = {
      ...notification,
      expiresAt: new Date(Date.now() + 3_600_000),
    };
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue(futureNotification);

    await worker.process(job() as any);

    expect(deliveryState.beginProcessing).toHaveBeenCalledWith(delivery);
    expect(deliveryState.complete).toHaveBeenCalledWith(
      delivery,
      futureNotification,
      { delivered: true },
    );
  });

  it('fails the delivery and rethrows on errors', async () => {
    deliveryRepository.findOne.mockResolvedValue(delivery);
    notificationRepository.findOne.mockResolvedValue(notification);
    const error = new Error('complete failed');
    deliveryState.complete.mockRejectedValue(error);

    await expect(worker.process(job() as any)).rejects.toThrow(
      'complete failed',
    );
    expect(deliveryState.fail).toHaveBeenCalledWith(
      delivery,
      notification,
      error,
    );
  });
});
