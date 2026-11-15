import { Injectable, Logger } from '@nestjs/common';
import { PresenceStore, PresenceState, PresenceRecord } from './presence.store';
import { NatsPublisher } from '../../infrastructure/nats/nats.publisher';
import { RealtimeNatsSubjects } from '@delivery-micro/shard';

@Injectable()
export class PresenceService {
  private readonly logger = new Logger(PresenceService.name);

  constructor(
    private readonly store: PresenceStore,
    private readonly natsPublisher: NatsPublisher,
  ) {}

  async setOnline(userId: string): Promise<void> {
    const previous = await this.store.get(userId);
    await this.store.setState(userId, 'ONLINE');
    if (!previous || previous.state !== 'ONLINE') {
      await this.broadcastPresence(userId, 'ONLINE');
    }
  }

  async setIdle(userId: string): Promise<void> {
    const previous = await this.store.get(userId);
    if (previous?.state === 'IDLE') return;
    await this.store.setState(userId, 'IDLE');
    await this.broadcastPresence(userId, 'IDLE');
  }

  async setOffline(userId: string): Promise<void> {
    const previous = await this.store.get(userId);
    await this.store.setState(userId, 'OFFLINE');
    if (!previous || previous.state !== 'OFFLINE') {
      await this.broadcastPresence(userId, 'OFFLINE');
    }
  }

  get(userId: string): Promise<PresenceRecord | null> {
    return this.store.get(userId);
  }

  private async broadcastPresence(
    userId: string,
    status: PresenceState,
  ): Promise<void> {
    await this.natsPublisher
      .publish(RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED, {
        driverId: userId,
        status,
        timestamp: new Date().toISOString(),
      })
      .catch((err) =>
        this.logger.warn(
          `Presence broadcast failed for ${userId}: ${err.message}`,
        ),
      );
  }
}
