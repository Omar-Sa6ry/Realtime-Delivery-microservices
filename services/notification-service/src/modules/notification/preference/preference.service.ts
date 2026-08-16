import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { NotificationPreference } from '../../../common/database/entities/notification-preference.entity';
import { NotificationType, NotificationChannel } from '@delivery/common';

@Injectable()
export class PreferenceService {
  private readonly logger = new Logger(PreferenceService.name);

  constructor(
    @InjectRepository(NotificationPreference)
    private preferenceRepository: Repository<NotificationPreference>,
  ) {}

  async getEnabledChannels(userId: string, type: NotificationType): Promise<NotificationChannel[]> {
    const preferences = await this.preferenceRepository.find({
      where: { userId, type },
    });

    const defaultChannels = [NotificationChannel.IN_APP, NotificationChannel.PUSH];

    if (preferences.length === 0) {
      // Return defaults if no preferences are explicitly set
      return defaultChannels;
    }

    const enabledChannels = preferences
      .filter((p) => p.enabled)
      .map((p) => p.channel);

    return enabledChannels;
  }

  async upsertPreferences(
    userId: string,
    type: NotificationType,
    channels: { channel: NotificationChannel; enabled: boolean }[],
  ): Promise<NotificationPreference[]> {
    const results: NotificationPreference[] = [];
    for (const c of channels) {
      let preference = await this.preferenceRepository.findOne({
        where: { userId, type, channel: c.channel },
      });

      if (!preference) {
        preference = this.preferenceRepository.create({
          userId,
          type,
          channel: c.channel,
          enabled: c.enabled,
        });
      } else {
        preference.enabled = c.enabled;
      }

      results.push(await this.preferenceRepository.save(preference));
    }
    return results;
  }

  async findForUser(userId: string, type?: NotificationType): Promise<NotificationPreference[]> {
    return this.preferenceRepository.find({
      where: type ? { userId, type } : { userId },
      order: { type: 'ASC', channel: 'ASC' },
    });
  }
}
