import { NotificationResolver } from './notification.resolver';
import { NotificationService } from './notification.service';
import { PreferenceService } from './preference/preference.service';
import { NotificationChannel, NotificationType } from '@delivery-micro/shard';
import type { IUser } from '@delivery-micro/shard';

describe('NotificationResolver', () => {
  let resolver: NotificationResolver;
  let notificationService: {
    findAllForUser: jest.Mock;
    findByIdForUser: jest.Mock;
    unreadCountForUser: jest.Mock;
    markAsRead: jest.Mock;
    markAllAsRead: jest.Mock;
    delete: jest.Mock;
  };
  let preferenceService: {
    findForUser: jest.Mock;
    upsertPreferences: jest.Mock;
  };
  let i18n: { t: jest.Mock };

  const user = { id: 'user-1' } as IUser;

  beforeEach(() => {
    notificationService = {
      findAllForUser: jest.fn(),
      findByIdForUser: jest.fn(),
      unreadCountForUser: jest.fn(),
      markAsRead: jest.fn(),
      markAllAsRead: jest.fn(),
      delete: jest.fn(),
    };
    preferenceService = {
      findForUser: jest.fn(),
      upsertPreferences: jest.fn(),
    };
    i18n = { t: jest.fn().mockResolvedValue('translated') };

    resolver = new NotificationResolver(
      notificationService as unknown as NotificationService,
      preferenceService as unknown as PreferenceService,
      i18n as any,
    );
  });

  describe('myNotifications', () => {
    it('returns a paginated connection with translated message', async () => {
      notificationService.findAllForUser.mockResolvedValue({
        items: [{ id: 'n-1' }],
        totalCount: 1,
      });

      const result = await resolver.myNotifications(user, 1, 20);

      expect(notificationService.findAllForUser).toHaveBeenCalledWith(
        'user-1',
        1,
        20,
      );
      expect(result).toMatchObject({
        success: true,
        statusCode: 200,
        message: 'translated',
        data: { items: [{ id: 'n-1' }], totalCount: 1 },
      });
      expect(i18n.t).toHaveBeenCalledWith('notification.LIST_RETRIEVED');
    });
  });

  describe('notification', () => {
    it('returns the requested notification', async () => {
      notificationService.findByIdForUser.mockResolvedValue({ id: 'n-1' });

      const result = await resolver.notification(user, 'n-1');

      expect(result).toMatchObject({
        success: true,
        data: { id: 'n-1' },
      });
      expect(i18n.t).toHaveBeenCalledWith('notification.GET_RETRIEVED');
    });

    it('returns undefined data when the notification is missing', async () => {
      notificationService.findByIdForUser.mockResolvedValue(null);

      const result = await resolver.notification(user, 'missing');

      expect(result.data).toBeUndefined();
    });
  });

  describe('unreadNotificationCount', () => {
    it('returns the unread count', async () => {
      notificationService.unreadCountForUser.mockResolvedValue(7);

      const result = await resolver.unreadNotificationCount(user);

      expect(result.data).toBe(7);
      expect(i18n.t).toHaveBeenCalledWith('notification.UNREAD_COUNT');
    });
  });

  describe('markNotificationAsRead', () => {
    it('marks a notification as read', async () => {
      const saved = { id: 'n-1', readAt: new Date() };
      notificationService.markAsRead.mockResolvedValue(saved);

      const result = await resolver.markNotificationAsRead(user, 'n-1');

      expect(notificationService.markAsRead).toHaveBeenCalledWith(
        'n-1',
        'user-1',
      );
      expect(result.data).toBe(saved);
      expect(i18n.t).toHaveBeenCalledWith('notification.MARKED_READ');
    });
  });

  describe('markAllNotificationsAsRead', () => {
    it('returns true after marking all as read', async () => {
      notificationService.markAllAsRead.mockResolvedValue(3);

      const result = await resolver.markAllNotificationsAsRead(user);

      expect(result).toMatchObject({ success: true, data: true });
      expect(i18n.t).toHaveBeenCalledWith('notification.ALL_MARKED_READ');
    });
  });

  describe('deleteNotification', () => {
    it('returns true after delete', async () => {
      notificationService.delete.mockResolvedValue(true);

      const result = await resolver.deleteNotification(user, 'n-1');

      expect(notificationService.delete).toHaveBeenCalledWith('n-1', 'user-1');
      expect(result.data).toBe(true);
      expect(i18n.t).toHaveBeenCalledWith('notification.DELETED');
    });
  });

  describe('myNotificationPreferences', () => {
    it('lists preferences for the user', async () => {
      const items = [
        { id: 'p-1', channel: NotificationChannel.EMAIL, enabled: true },
      ];
      preferenceService.findForUser.mockResolvedValue(items);

      const result = await resolver.myNotificationPreferences(
        user,
        NotificationType.DELIVERY_CREATED,
      );

      expect(preferenceService.findForUser).toHaveBeenCalledWith(
        'user-1',
        NotificationType.DELIVERY_CREATED,
      );
      expect(result.items).toEqual(items);
      expect(i18n.t).toHaveBeenCalledWith('notification.PREFERENCES_RETRIEVED');
    });

    it('supports listing preferences without a type filter', async () => {
      preferenceService.findForUser.mockResolvedValue([]);

      await resolver.myNotificationPreferences(user, undefined);

      expect(preferenceService.findForUser).toHaveBeenCalledWith(
        'user-1',
        undefined,
      );
    });
  });

  describe('updateNotificationPreferences', () => {
    it('upserts every preference group and aggregates results', async () => {
      const rowsA = [{ id: 'p-1' }];
      const rowsB = [{ id: 'p-2' }, { id: 'p-3' }];
      preferenceService.upsertPreferences
        .mockResolvedValueOnce(rowsA)
        .mockResolvedValueOnce(rowsB);

      const result = await resolver.updateNotificationPreferences(user, [
        {
          type: NotificationType.DELIVERY_CREATED,
          channels: [{ channel: NotificationChannel.EMAIL, enabled: true }],
        },
        {
          type: NotificationType.PAYMENT_COMPLETED,
          channels: [
            { channel: NotificationChannel.SMS, enabled: false },
            { channel: NotificationChannel.PUSH, enabled: true },
          ],
        },
      ]);

      expect(preferenceService.upsertPreferences).toHaveBeenCalledTimes(2);
      expect(result.items).toHaveLength(3);
      expect(i18n.t).toHaveBeenCalledWith('notification.PREFERENCES_UPDATED');
    });

    it('handles an empty preferences payload', async () => {
      const result = await resolver.updateNotificationPreferences(user, []);

      expect(result.items).toEqual([]);
      expect(preferenceService.upsertPreferences).not.toHaveBeenCalled();
    });
  });
});
