import { RetryWorker } from './retry.worker';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { NotificationDispatcherService } from '../notification/notification-dispatcher.service';
import { DeliveryStateService } from './delivery-state.service';
import {
  DeliveryChannelStatus,
  NotificationStatus,
  NotificationType,
} from '@delivery-micro/shard';
import {
  createMockQueue,
  createMockRepository,
  MockQueue,
  MockRepository,
} from '../../test-utils';

describe('RetryWorker', () => {
  let worker: RetryWorker;
  let retryQueue: MockQueue;
  let notificationRepository: MockRepository<Notification>;
  let deliveryRepository: MockRepository<NotificationDelivery>;
  let dispatcherService: jest.Mocked<NotificationDispatcherService>;
  let deliveryState: jest.Mocked<DeliveryStateService>;

  beforeEach(() => {
    retryQueue = createMockQueue();
    notificationRepository = createMockRepository<Notification>();
    deliveryRepository = createMockRepository<NotificationDelivery>();
    dispatcherService = {
      dispatch: jest.fn(),
      schedule: jest.fn(),
      requeue: jest.fn().mockResolvedValue(undefined),
    } as any;
    deliveryState = {
      expire: jest.fn().mockResolvedValue(undefined),
    } as any;

    worker = new RetryWorker(
      retryQueue as any,
      notificationRepository as any,
      deliveryRepository as any,
      dispatcherService,
      deliveryState,
    );
  });

  describe('onModuleInit', () => {
    it('registers the periodic sweep job scheduler', async () => {
      await worker.onModuleInit();

      expect(retryQueue.upsertJobScheduler).toHaveBeenCalledWith(
        'notification-sweep',
        { every: 60_000 },
        { name: 'sweep', data: {} },
      );
    });
  });

  describe('process', () => {
    it('ignores jobs that are not the sweep job', async () => {
      await worker.process({ name: 'other', data: {} } as any);

      expect(deliveryState.expire).not.toHaveBeenCalled();
      expect(dispatcherService.requeue).not.toHaveBeenCalled();
    });

    it('expires due notifications during the sweep', async () => {
      const due = {
        id: 'n-due',
        status: NotificationStatus.QUEUED,
        type: NotificationType.DELIVERY_CREATED,
        userId: 'u1',
        title: 't',
        body: 'b',
      } as Notification;
      notificationRepository.find.mockResolvedValue([due]);
      deliveryRepository.find.mockResolvedValue([]);

      await worker.process({ name: 'sweep', data: {} } as any);

      expect(notificationRepository.find).toHaveBeenCalledWith(
        expect.objectContaining({
          where: expect.objectContaining({
            status: expect.anything(),
            expiresAt: expect.anything(),
          }),
        }),
      );
      expect(deliveryState.expire).toHaveBeenCalledWith(due);
    });

    it('requeues failed deliveries that are eligible for retry', async () => {
      notificationRepository.find.mockResolvedValue([]);
      const failedDelivery = {
        id: 'd-failed',
        notificationId: 'n-1',
        channel: 'EMAIL',
        status: DeliveryChannelStatus.FAILED,
        attemptCount: 1,
      } as NotificationDelivery;
      deliveryRepository.find.mockResolvedValue([failedDelivery]);
      deliveryRepository.save.mockImplementation(async (d: any) => d);

      await worker.process({ name: 'sweep', data: {} } as any);

      expect(failedDelivery.status).toBe(DeliveryChannelStatus.PENDING);
      expect(deliveryRepository.save).toHaveBeenCalledWith(failedDelivery);
      expect(dispatcherService.requeue).toHaveBeenCalledWith(failedDelivery);
    });

    it('does nothing when nothing is due or failed', async () => {
      notificationRepository.find.mockResolvedValue([]);
      deliveryRepository.find.mockResolvedValue([]);

      await worker.process({ name: 'sweep', data: {} } as any);

      expect(deliveryState.expire).not.toHaveBeenCalled();
      expect(dispatcherService.requeue).not.toHaveBeenCalled();
    });
  });
});
