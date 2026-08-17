import { Injectable, Logger } from '@nestjs/common';
import { GrpcClient } from '../../grpc/grpc.client';
import { RedisService } from '@bts-soft/cache';
import { redisKeys, TTL } from '../../../common/common-types/constants';

/**
 * Delivery authorization policy — resolves participant status through the
 * delivery domain service over gRPC (circuit-broken + cached 30s).
 */
@Injectable()
export class DeliveryPolicy {
  private readonly logger = new Logger(DeliveryPolicy.name);

  constructor(
    private readonly grpc: GrpcClient,
    private readonly redis: RedisService,
  ) {}

  async isParticipant(userId: string, deliveryId: string): Promise<boolean> {
    const cacheKey = redisKeys.authzCache(userId, deliveryId);
    const cached = await this.redis
      .get<{ isParticipant: boolean }>(cacheKey)
      .catch(() => null);

    if (cached) return cached.isParticipant;

    if (!this.grpc.isDeliveryClientConfigured()) {
      this.logger.warn(
        `Delivery gRPC endpoint not configured; denying participant check userId=${userId} delivery=${deliveryId}`,
      );
      return false;
    }

    try {
      const res = await this.grpc.callDelivery('IsParticipant', {
        userId,
        deliveryId,
      });
      const isParticipant = res?.isParticipant === true;
      if (isParticipant) {
        await this.redis
          .set(cacheKey, { isParticipant: true }, TTL.AUTHZ_CACHE)
          .catch(() => undefined);
      }
      return isParticipant;
    } catch (err) {
      this.logger.error(
        `DeliveryService.IsParticipant failed (userId=${userId} delivery=${deliveryId}): ${err.message}`,
      );
      return false;
    }
  }
}