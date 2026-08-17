import { Injectable, Logger } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { redisKeys, TTL } from '../../../common/common-types/constants';

export type PresenceState = 'ONLINE' | 'IDLE' | 'OFFLINE';

export interface PresenceRecord {
  state: PresenceState;
  lastSeen: number;
}

@Injectable()
export class PresenceStore {
  private readonly logger = new Logger(PresenceStore.name);

  constructor(private readonly redis: RedisService) {}

  async setState(userId: string, state: PresenceState): Promise<void> {
    const record: PresenceRecord = { state, lastSeen: Date.now() };
    await this.redis
      .set(redisKeys.presence(userId), record, TTL.PRESENCE)
      .catch((err) =>
        this.logger.error(`Failed to set presence for ${userId}: ${err.message}`),
      );
  }

  async get(userId: string): Promise<PresenceRecord | null> {
    return await this.redis
      .get<PresenceRecord>(redisKeys.presence(userId))
      .catch(() => null);
  }
}