import { PreferenceService } from './preference.service';
import { NotificationPreference } from '../../../common/database/entities/notification-preference.entity';
import { NotificationChannel, NotificationType } from '@delivery-micro/shard';
import { createMockRepository, MockRepository } from '../../../test-utils';

describe('PreferenceService', () => {
  let service: PreferenceService;
  let repository: MockRepository<NotificationPreference>;

  beforeEach(() => {
    repository = createMockRepository<NotificationPreference>();
    service = new PreferenceService(repository as any);
  });

  describe('getEnabledChannels', () => {
    it('returns default channels (IN_APP, PUSH) when no preferences exist', async () => {
      repository.find.mockResolvedValue([]);

      const channels = await service.getEnabledChannels(
        'user-1',
        NotificationType.DELIVERY_CREATED,
      );

      expect(channels).toEqual([
        NotificationChannel.IN_APP,
        NotificationChannel.PUSH,
      ]);
      expect(repository.find).toHaveBeenCalledWith({
        where: { userId: 'user-1', type: NotificationType.DELIVERY_CREATED },
      });
    });

    it('returns only enabled channels when preferences exist', async () => {
      repository.find.mockResolvedValue([
        {
          channel: NotificationChannel.EMAIL,
          enabled: true,
        },
        {
          channel: NotificationChannel.SMS,
          enabled: false,
        },
        {
          channel: NotificationChannel.PUSH,
          enabled: true,
        },
      ]);

      const channels = await service.getEnabledChannels(
        'user-1',
        NotificationType.DELIVERY_CREATED,
      );

      expect(channels).toEqual([
        NotificationChannel.EMAIL,
        NotificationChannel.PUSH,
      ]);
    });

    it('returns empty array when all preferences are disabled', async () => {
      repository.find.mockResolvedValue([
        { channel: NotificationChannel.EMAIL, enabled: false },
        { channel: NotificationChannel.PUSH, enabled: false },
      ]);

      const channels = await service.getEnabledChannels(
        'user-1',
        NotificationType.DELIVERY_CREATED,
      );

      expect(channels).toEqual([]);
    });
  });

  describe('upsertPreferences', () => {
    it('creates and saves a new preference when none exists', async () => {
      repository.findOne.mockResolvedValue(null);
      repository.save.mockImplementation(async (p: any) => ({
        ...p,
        id: 'pref-1',
      }));

      const results = await service.upsertPreferences(
        'user-1',
        NotificationType.DELIVERY_CREATED,
        [{ channel: NotificationChannel.EMAIL, enabled: true }],
      );

      expect(repository.create).toHaveBeenCalledWith({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        channel: NotificationChannel.EMAIL,
        enabled: true,
      });
      expect(repository.save).toHaveBeenCalledTimes(1);
      expect(results).toHaveLength(1);
      expect(results[0].id).toBe('pref-1');
    });

    it('updates an existing preference enabled flag', async () => {
      const existing = {
        id: 'pref-existing',
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        channel: NotificationChannel.EMAIL,
        enabled: true,
      };
      repository.findOne.mockResolvedValue(existing);
      repository.save.mockImplementation(async (p: any) => p);

      const results = await service.upsertPreferences(
        'user-1',
        NotificationType.DELIVERY_CREATED,
        [{ channel: NotificationChannel.EMAIL, enabled: false }],
      );

      expect(repository.create).not.toHaveBeenCalled();
      expect(existing.enabled).toBe(false);
      expect(results[0]).toBe(existing);
    });

    it('handles multiple channels in a single upsert', async () => {
      repository.findOne.mockResolvedValue(null);
      repository.save.mockImplementation(async (p: any) => p);

      const results = await service.upsertPreferences(
        'user-1',
        NotificationType.DELIVERY_CREATED,
        [
          { channel: NotificationChannel.EMAIL, enabled: true },
          { channel: NotificationChannel.SMS, enabled: false },
          { channel: NotificationChannel.PUSH, enabled: true },
        ],
      );

      expect(results).toHaveLength(3);
      expect(repository.save).toHaveBeenCalledTimes(3);
    });
  });

  describe('findForUser', () => {
    it('filters by type when provided', async () => {
      repository.find.mockResolvedValue([]);

      await service.findForUser('user-1', NotificationType.PAYMENT_COMPLETED);

      expect(repository.find).toHaveBeenCalledWith({
        where: { userId: 'user-1', type: NotificationType.PAYMENT_COMPLETED },
        order: { type: 'ASC', channel: 'ASC' },
      });
    });

    it('does not filter by type when omitted', async () => {
      repository.find.mockResolvedValue([]);

      await service.findForUser('user-1');

      expect(repository.find).toHaveBeenCalledWith({
        where: { userId: 'user-1' },
        order: { type: 'ASC', channel: 'ASC' },
      });
    });
  });
});
