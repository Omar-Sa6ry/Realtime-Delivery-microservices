import { Injectable, Logger } from '@nestjs/common';
import { RedisService } from '@bts-soft/cache';
import { redisKeys } from '../../common/common-types/constants';

export interface SubscriptionContext {
  socketId: string;
  deliveryId: string;
  subscribedAt: number;
}

@Injectable()
export class SubscriptionStore {
  private readonly logger = new Logger(SubscriptionStore.name);

  constructor(private readonly redis: RedisService) {}

  async addSubscriber(deliveryId: string, socketId: string): Promise<void> {
    await this.redis.sAdd(redisKeys.deliverySubscribers(deliveryId), socketId);
    await this.redis.sAdd(redisKeys.socketSubscriptions(socketId), deliveryId);
  }

  async removeSubscriber(deliveryId: string, socketId: string): Promise<void> {
    await this.redis.sRem(redisKeys.deliverySubscribers(deliveryId), socketId);
    await this.redis.sRem(redisKeys.socketSubscriptions(socketId), deliveryId);
  }

  async getDeliverySubscribers(deliveryId: string): Promise<string[]> {
    return await this.redis.sMembers(redisKeys.deliverySubscribers(deliveryId));
  }

  async isSubscribed(deliveryId: string, socketId: string): Promise<boolean> {
    return await this.redis.sIsMember(redisKeys.deliverySubscribers(deliveryId), socketId);
  }

  /**
   * Removes a socket from every delivery it subscribed to (disconnect cleanup).
   * Relies on the reverse index ws:socket:{socketId}:subscriptions.
   */
  async removeSocketFromAllSubscriptions(socketId: string): Promise<string[]> {
    const deliveries = await this.redis.sMembers(redisKeys.socketSubscriptions(socketId));
    for (const deliveryId of deliveries) {
      await this.redis.sRem(redisKeys.deliverySubscribers(deliveryId), socketId);
    }
    await this.redis.del(redisKeys.socketSubscriptions(socketId));
    return deliveries;
  }
}