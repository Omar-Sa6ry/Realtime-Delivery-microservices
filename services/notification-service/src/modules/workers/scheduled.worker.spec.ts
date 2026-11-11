import { ScheduledWorker } from './scheduled.worker';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { NotificationDispatcherService } from '../notification/notification-dispatcher.service';
import { DeliveryChannelStatus, NotificationType } from '@delivery-micro/shard';
import { createMockRepository, MockRepository } from '../../test-utils';

describe('ScheduledWorker', () => {
  let worker: ScheduledWorker;
  let notificationRepository: MockRepository<Notification>;
  let deliveryRepository: MockRepository<NotificationDelivery>;
  let dispatcherService: jest.Mocked<NotificationDispatcherService>;

  const job = { data: { notificationId: 'n-1' } };

  beforeEach(() => {
    notificationRepository = createMockRepository<Notification>();
    deliveryRepository = createMockRepository<NotificationDelivery>();
    dispatcherService = {
      dispatch: jest.fn().mockResolvedValue(undefined),
      schedule: jest.fn(),
      requeue: jest.fn(),
    } as any;

    worker = new ScheduledWorker(
      notificationRepository as any,
      deliveryRepository as any,
      dispatcherService,
    );
  });

  it('returns quietly when the notification no longer exists', async () => {
    notificationRepository.findOne.mockResolvedValue(null);

    await expect(worker.process(job as any)).resolves.toBeUndefined();
    expect(dispatcherService.dispatch).not.toHaveBeenCalled();
  });

  it('returns quietly when there are no pending deliveries', async () => {
    notificationRepository.findOne.mockResolvedValue({
      id: 'n-1',
      type: NotificationType.DELIVERY_CREATED,
    } as Notification);
    deliveryRepository.find.mockResolvedValue([]);

    await expect(worker.process(job as any)).resolves.toBeUndefined();
    expect(dispatcherService.dispatch).not.toHaveBeenCalled();
  });

  it('dispatches pending deliveries for a due notification', async () => {
    const notification = {
      id: 'n-1',
      type: NotificationType.DELIVERY_CREATED,
    } as Notification;
    const deliveries = [
      {
        id: 'd-1',
        notificationId: 'n-1',
        status: DeliveryChannelStatus.PENDING,
      },
    ] as NotificationDelivery[];
    notificationRepository.findOne.mockResolvedValue(notification);
    deliveryRepository.find.mockResolvedValue(deliveries);

    await worker.process(job as any);

    expect(deliveryRepository.find).toHaveBeenCalledWith({
      where: {
        notificationId: 'n-1',
        status: expect.anything(),
      },
    });
    expect(dispatcherService.dispatch).toHaveBeenCalledWith(
      notification,
      deliveries,
    );
  });
});
