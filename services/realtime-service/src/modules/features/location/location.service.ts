import { Injectable, Logger } from '@nestjs/common';
import { I18nService, I18nContext } from 'nestjs-i18n';
import { RedisService } from '@bts-soft/cache';
import { Role, RealtimeNatsSubjects, ServerMessageType } from '@delivery/common';
import { WsErrorCode, WsException } from '@delivery/common';
import { redisKeys, TTL } from '../../../common/common-types/constants';
import { LocationValidator } from './location.validator';
import { LocationThrottler } from './location.throttler';
import { LocationUpdatePayload, LocationUpdateResult } from './location.types';
import { RealtimeAuthorizationService } from '../../gateway/authorization/realtime-authorization.service';
import { NatsPublisher } from '../../infrastructure/nats/nats.publisher';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import { AuthenticatedSocket } from '../../gateway/connection/connection.types';
import { SocketWriter } from '../../gateway/connection/socket-writer.service';
import { MessagePriority } from '@delivery/common';

@Injectable()
export class LocationService {
  private readonly logger = new Logger(LocationService.name);

  constructor(
    private readonly validator: LocationValidator,
    private readonly throttler: LocationThrottler,
    private readonly authorization: RealtimeAuthorizationService,
    private readonly natsPublisher: NatsPublisher,
    private readonly redis: RedisService,
    private readonly i18n: I18nService,
    private readonly metrics: RealtimeMetricsService,
    private readonly writer: SocketWriter,
  ) {}

  /**
   * Full location pipeline for a driver:
   *  validate -> authorize -> throttle -> store (Redis) -> publish (NATS).
   */
  async handle(
    socket: AuthenticatedSocket,
    payload: LocationUpdatePayload,
    requestId?: string,
  ): Promise<void> {
    // 1. Validate
    const result: LocationUpdateResult = this.validator.validate(payload);
    if (!result.valid) {
      throw new WsException(
        WsErrorCode.INVALID_MESSAGE,
        result.errors?.join('; ') || 'Invalid location payload',
        false,
      );
    }

    // 2. Authorize (driver only + assigned to this delivery)
    if (socket.data.userRole !== Role.DRIVER) {
      throw new WsException(WsErrorCode.FORBIDDEN, this.t('ws.unauthorized'), false);
    }
    const authorized = await this.authorization.canSendLocationUpdate(
      socket.data.userId,
      result.deliveryId!,
    );
    if (!authorized) {
      this.metrics.locationUpdates.inc({ result: 'rejected_unauthorized' });
      throw new WsException(WsErrorCode.FORBIDDEN, this.t('ws.notAssignedDriver'), false);
    }

    // 3. Throttle (token bucket)
    const allowed = await this.throttler.allow(socket.data.userId);
    if (!allowed) {
      this.metrics.locationUpdates.inc({ result: 'rejected_rate_limited' });
      await this.writer.send(
        socket,
        {
          requestId,
          type: ServerMessageType.LOCATION_UPDATE_REJECTED,
          data: {
            deliveryId: result.deliveryId!,
            reason: WsErrorCode.RATE_LIMITED,
          },
        },
        MessagePriority.NORMAL,
      );
      return;
    }

    // 4. Store latest-value-wins snapshot
    await this.redis
      .set(
        redisKeys.driverLocation(socket.data.userId),
        {
          lat: result.lat,
          lng: result.lng,
          accuracy: result.accuracy,
          speed: result.speed,
          heading: result.heading,
          timestamp: new Date(result.timestamp!).toISOString(),
          deliveryId: result.deliveryId,
        },
        TTL.DRIVER_LOCATION,
      )
      .catch((err) => this.logger.warn(`Failed to store driver location: ${err.message}`));

    // 5. Fan-out via NATS (all nodes, filtered to delivery subscribers)
    await this.natsPublisher.publish(RealtimeNatsSubjects.DELIVERY_LOCATION_UPDATED, {
      deliveryId: result.deliveryId,
      driverId: socket.data.userId,
      lat: result.lat,
      lng: result.lng,
      accuracy: result.accuracy,
      speed: result.speed,
      heading: result.heading,
      timestamp: new Date(result.timestamp!).toISOString(),
    });

    this.metrics.locationUpdates.inc({ result: 'accepted' });
  }

  private t(key: string): string {
    const lang = I18nContext.current()?.lang || 'en';
    return String(this.i18n.t(key, { lang })) || key;
  }
}