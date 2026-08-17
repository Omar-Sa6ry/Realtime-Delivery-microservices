import { Injectable, Logger } from '@nestjs/common';
import { GrpcClient } from '../../../infrastructure/grpc/grpc.client';
import { RedisService } from '@bts-soft/cache';
import { redisKeys, TTL } from '../../../../common/common-types/constants';

/**
 * Driver authorization policy — resolves assigned-driver status through the
 * driver domain service over gRPC (circuit-broken + cached 30s).
 */
@Injectable()
export class DriverPolicy {
  private readonly logger = new Logger(DriverPolicy.name);

  constructor(
    private readonly grpc: GrpcClient,
    private readonly redis: RedisService,
  ) {}

  async isAssignedDriver(driverId: string, deliveryId: string): Promise<boolean> {
    const cacheKey = redisKeys.authzCache(`driver:${driverId}`, deliveryId);
    const cached = await this.redis
      .get<{ isAssigned: boolean }>(cacheKey)
      .catch(() => null);

    if (cached) return cached.isAssigned;

    if (!this.grpc.isDriverClientConfigured()) {
      this.logger.warn(
        `Driver gRPC endpoint not configured; denying assignment check driverId=${driverId} delivery=${deliveryId}`,
      );
      return false;
    }

    try {
      const res = await this.grpc.callDriver('IsAssignedDriver', {
        driverId,
        deliveryId,
      });
      const isAssigned = res?.isAssigned === true;
      if (isAssigned) {
        await this.redis
          .set(cacheKey, { isAssigned: true }, TTL.AUTHZ_CACHE)
          .catch(() => undefined);
      }
      return isAssigned;
    } catch (err) {
      this.logger.error(
        `DriverService.IsAssignedDriver failed (driverId=${driverId} delivery=${deliveryId}): ${err.message}`,
      );
      return false;
    }
  }
}