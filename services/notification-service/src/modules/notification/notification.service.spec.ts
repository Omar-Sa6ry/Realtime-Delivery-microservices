import { NotificationService } from './notification.service';
import { Notification } from '../../common/database/entities/notification.entity';
import { NotificationDelivery } from '../../common/database/entities/notification-delivery.entity';
import { NotificationDispatcherService } from './notification-dispatcher.service';
import { TemplateService } from './template/template.service';
import { PreferenceService } from './preference/preference.service';
import {
  NotificationChannel,
  NotificationPriority,
  NotificationStatus,
  NotificationType,
} from '@delivery-micro/shard';
import {
  createMockDataSource,
  createMockQueryRunner,
  createMockRepository,
  MockQueryRunner,
  MockRepository,
} from '../../test-utils';

describe('NotificationService', () => {
  let service: NotificationService;
  let notificationRepository: MockRepository<Notification>;
  let dispatcherService: jest.Mocked<NotificationDispatcherService>;
  let templateService: jest.Mocked<TemplateService>;
  let preferenceService: jest.Mocked<PreferenceService>;
  let queryRunner: MockQueryRunner;
  let i18n: { t: jest.Mock };

  beforeEach(() => {
    notificationRepository = createMockRepository<Notification>();
    dispatcherService = {
      dispatch: jest.fn().mockResolvedValue(undefined),
      schedule: jest.fn().mockResolvedValue(undefined),
      requeue: jest.fn().mockResolvedValue(undefined),
    } as any;
    templateService = {
      render: jest.fn().mockResolvedValue({
        title: 'Rendered title',
        body: 'Rendered body',
      }),
    } as any;
    preferenceService = {
      getEnabledChannels: jest
        .fn()
        .mockResolvedValue([
          NotificationChannel.IN_APP,
          NotificationChannel.PUSH,
        ]),
    } as any;
    queryRunner = createMockQueryRunner();
    const dataSource = createMockDataSource(queryRunner);
    i18n = { t: jest.fn().mockResolvedValue('Notification not found') };

    service = new NotificationService(
      notificationRepository as any,
      dispatcherService,
      templateService,
      preferenceService,
      dataSource as any,
      i18n as any,
    );
  });

  describe('createAndDispatch', () => {
    it('returns null when no channels are enabled', async () => {
      preferenceService.getEnabledChannels.mockResolvedValue([]);

      const result = await service.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
      });

      expect(result).toBeNull();
      expect(queryRunner.startTransaction).not.toHaveBeenCalled();
    });

    it('uses requiredChannels to bypass preference lookups', async () => {
      queryRunner.manager.create.mockImplementation((_e: any, data: any) => ({
        ...data,
        id: 'created-1',
      }));

      await service.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.PASSWORD_RESET_REQUESTED,
        requiredChannels: [NotificationChannel.IN_APP],
      });

      expect(preferenceService.getEnabledChannels).not.toHaveBeenCalled();
      expect(templateService.render).toHaveBeenCalledWith(
        NotificationType.PASSWORD_RESET_REQUESTED,
        NotificationChannel.IN_APP,
        'en',
        {},
      );
      expect(queryRunner.manager.create).toHaveBeenCalledWith(
        NotificationDelivery,
        expect.objectContaining({ channel: NotificationChannel.IN_APP }),
      );
    });

    it('opens a transaction, persists notification + deliveries, commits and dispatches', async () => {
      queryRunner.manager.create.mockImplementation(
        (_e: any, data: any, idx?: number) => ({
          ...data,
          id:
            data?.id ??
            `created-${queryRunner.manager.create.mock.calls.length}`,
        }),
      );

      const result = await service.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        data: { orderId: 'o-1' },
      });

      expect(queryRunner.connect).toHaveBeenCalled();
      expect(queryRunner.startTransaction).toHaveBeenCalled();
      expect(queryRunner.commitTransaction).toHaveBeenCalled();
      expect(queryRunner.rollbackTransaction).not.toHaveBeenCalled();
      expect(queryRunner.release).toHaveBeenCalled();
      expect(result).toBe('created-1');
      expect(dispatcherService.dispatch).toHaveBeenCalledTimes(1);
      expect(dispatcherService.schedule).not.toHaveBeenCalled();
      expect(queryRunner.manager.create).toHaveBeenCalledWith(
        Notification,
        expect.objectContaining({
          userId: 'user-1',
          type: NotificationType.DELIVERY_CREATED,
          title: 'Rendered title',
          body: 'Rendered body',
          priority: NotificationPriority.NORMAL,
          data: { orderId: 'o-1' },
        }),
      );
      expect(queryRunner.manager.save).toHaveBeenCalledTimes(2);
    });

    it('prefers explicit title/body over rendered template output', async () => {
      await service.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        title: 'Custom title',
        body: 'Custom body',
      });

      expect(queryRunner.manager.create).toHaveBeenCalledWith(
        Notification,
        expect.objectContaining({
          title: 'Custom title',
          body: 'Custom body',
        }),
      );
    });

    it('routes to dispatcher.schedule when scheduledAt is in the future', async () => {
      const scheduledAt = new Date(Date.now() + 3_600_000);

      await service.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        scheduledAt,
      });

      expect(dispatcherService.schedule).toHaveBeenCalledWith(
        expect.anything(),
        scheduledAt,
      );
      expect(dispatcherService.dispatch).not.toHaveBeenCalled();
    });

    it('dispatches immediately when scheduledAt is in the past', async () => {
      const scheduledAt = new Date(Date.now() - 1000);

      await service.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        scheduledAt,
      });

      expect(dispatcherService.dispatch).toHaveBeenCalled();
      expect(dispatcherService.schedule).not.toHaveBeenCalled();
    });

    it('rolls back and returns null on persistence errors', async () => {
      queryRunner.manager.save.mockRejectedValue(new Error('db failure'));

      const result = await service.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
      });

      expect(result).toBeNull();
      expect(queryRunner.rollbackTransaction).toHaveBeenCalled();
      expect(queryRunner.commitTransaction).not.toHaveBeenCalled();
      expect(queryRunner.release).toHaveBeenCalled();
      expect(dispatcherService.dispatch).not.toHaveBeenCalled();
    });
  });

  describe('findAllForUser', () => {
    it('paginates with clamped page/limit and relations', async () => {
      notificationRepository.findAndCount.mockResolvedValue([
        [{ id: 'n-1' }],
        1,
      ]);

      const result = await service.findAllForUser('user-1', 2, 50);

      expect(notificationRepository.findAndCount).toHaveBeenCalledWith({
        where: { userId: 'user-1' },
        relations: { deliveries: true },
        order: { createdAt: 'DESC' },
        skip: 50,
        take: 50,
      });
      expect(result).toEqual({ items: [{ id: 'n-1' }], totalCount: 1 });
    });

    it('clamps limit to [1, 100] and page to >= 1', async () => {
      await service.findAllForUser('user-1', 0, 500);

      expect(notificationRepository.findAndCount).toHaveBeenCalledWith(
        expect.objectContaining({ skip: 0, take: 100 }),
      );
    });
  });

  describe('findByIdForUser', () => {
    it('finds by id and userId with deliveries relation', async () => {
      notificationRepository.findOne.mockResolvedValue({ id: 'n-1' });

      const result = await service.findByIdForUser('n-1', 'user-1');

      expect(notificationRepository.findOne).toHaveBeenCalledWith({
        where: { id: 'n-1', userId: 'user-1' },
        relations: { deliveries: true },
      });
      expect(result).toEqual({ id: 'n-1' });
    });

    it('returns null when not found', async () => {
      notificationRepository.findOne.mockResolvedValue(null);

      await expect(
        service.findByIdForUser('missing', 'user-1'),
      ).resolves.toBeNull();
    });
  });

  describe('unreadCountForUser', () => {
    it('counts unread notifications using IsNull filter', async () => {
      notificationRepository.count.mockResolvedValue(5);

      const count = await service.unreadCountForUser('user-1');

      expect(count).toBe(5);
      const where = notificationRepository.count.mock.calls[0][0].where;
      expect(where.userId).toBe('user-1');
      expect(where.readAt).toBeDefined();
    });
  });

  describe('markAsRead', () => {
    it('throws a translated NotFoundException when missing', async () => {
      notificationRepository.findOne.mockResolvedValue(null);

      await expect(service.markAsRead('missing', 'user-1')).rejects.toThrow(
        'Notification not found',
      );
      expect(i18n.t).toHaveBeenCalledWith('notification.NOT_FOUND');
    });

    it('sets readAt and saves the notification', async () => {
      const existing = {
        id: 'n-1',
        userId: 'user-1',
        status: NotificationStatus.CREATED,
        readAt: null,
      };
      notificationRepository.findOne.mockResolvedValue(existing);
      notificationRepository.save.mockImplementation(async (n: any) => n);

      const result = await service.markAsRead('n-1', 'user-1');

      expect(existing.readAt).toBeInstanceOf(Date);
      expect(notificationRepository.save).toHaveBeenCalledWith(existing);
      expect(result).toBe(existing);
    });
  });

  describe('markAllAsRead', () => {
    it('updates all unread rows and returns the affected count', async () => {
      notificationRepository.update.mockResolvedValue({ affected: 3 });

      const affected = await service.markAllAsRead('user-1');

      expect(affected).toBe(3);
      expect(notificationRepository.update).toHaveBeenCalledTimes(1);
      const [filter] = notificationRepository.update.mock.calls[0];
      expect(filter.userId).toBe('user-1');
    });

    it('returns 0 when affected is undefined', async () => {
      notificationRepository.update.mockResolvedValue({});

      await expect(service.markAllAsRead('user-1')).resolves.toBe(0);
    });
  });

  describe('delete', () => {
    it('returns true when a row was deleted', async () => {
      notificationRepository.delete.mockResolvedValue({ affected: 1 });

      await expect(service.delete('n-1', 'user-1')).resolves.toBe(true);
    });

    it('returns false when nothing was deleted', async () => {
      notificationRepository.delete.mockResolvedValue({ affected: 0 });

      await expect(service.delete('missing', 'user-1')).resolves.toBe(false);
    });

    it('returns false when affected is undefined', async () => {
      notificationRepository.delete.mockResolvedValue({});

      await expect(service.delete('n-1', 'user-1')).resolves.toBe(false);
    });
  });
});
