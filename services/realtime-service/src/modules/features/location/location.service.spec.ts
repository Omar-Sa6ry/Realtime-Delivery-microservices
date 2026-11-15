import { LocationService } from './location.service';
import { LocationValidator } from './location.validator';
import { LocationThrottler } from './location.throttler';
import { RealtimeAuthorizationService } from '../../gateway/authorization/realtime-authorization.service';
import { NatsPublisher } from '../../infrastructure/nats/nats.publisher';
import { RedisService } from '@bts-soft/cache';
import { I18nService } from 'nestjs-i18n';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import { SocketWriter } from '../../gateway/connection/socket-writer.service';
import { AuthenticatedSocket } from '../../gateway/connection/connection.types';
import {
  MessagePriority,
  RealtimeNatsSubjects,
  Role,
  ServerMessageType,
  WsErrorCode,
  WsException,
} from '@delivery-micro/shard';
import { LocationUpdatePayload } from './location.types';

describe('LocationService', () => {
  let service: LocationService;
  let validator: LocationValidator;
  let throttler: { allow: jest.Mock };
  let authorization: { canSendLocationUpdate: jest.Mock };
  let natsPublisher: { publish: jest.Mock };
  let redis: { geoAdd: jest.Mock; set: jest.Mock };
  let i18n: { t: jest.Mock };
  let metrics: {
    locationUpdates: { inc: jest.Mock };
  };
  let writer: { send: jest.Mock };

  const socket = (role: Role = Role.DRIVER, userId = 'driver-1') =>
    ({ data: { userId, userRole: role } }) as unknown as AuthenticatedSocket;

  const payload = (): LocationUpdatePayload => ({
    deliveryId: 'delivery_1234',
    lat: 30.5,
    lng: 31.2,
    accuracy: 4,
    speed: 10,
    heading: 45,
    timestamp: new Date().toISOString(),
  });

  beforeEach(() => {
    validator = new LocationValidator();
    throttler = { allow: jest.fn().mockResolvedValue(true) };
    authorization = {
      canSendLocationUpdate: jest.fn().mockResolvedValue(true),
    };
    natsPublisher = { publish: jest.fn().mockResolvedValue(true) };
    redis = {
      geoAdd: jest.fn().mockResolvedValue(1),
      set: jest.fn().mockResolvedValue(undefined),
    };
    i18n = { t: jest.fn((key: string) => `t:${key}`) };
    metrics = { locationUpdates: { inc: jest.fn() } };
    writer = { send: jest.fn() };

    service = new LocationService(
      validator,
      throttler as unknown as LocationThrottler,
      authorization as unknown as RealtimeAuthorizationService,
      natsPublisher as unknown as NatsPublisher,
      redis as unknown as RedisService,
      i18n as unknown as I18nService,
      metrics as unknown as RealtimeMetricsService,
      writer as unknown as SocketWriter,
    );
  });

  it('rejects an invalid payload before anything else', async () => {
    await expect(
      service.handle(socket(), { ...payload(), lat: 999 }),
    ).rejects.toMatchObject({ code: WsErrorCode.INVALID_MESSAGE });
    expect(authorization.canSendLocationUpdate).not.toHaveBeenCalled();
    expect(natsPublisher.publish).not.toHaveBeenCalled();
  });

  it('rejects non-driver users with a translated forbidden error', async () => {
    await expect(service.handle(socket(Role.USER), payload())).rejects.toThrow(
      WsException,
    );
    await expect(
      service.handle(socket(Role.USER), payload()),
    ).rejects.toMatchObject({ code: WsErrorCode.FORBIDDEN });
    expect(i18n.t).toHaveBeenCalledWith('messages.ws.unauthorized', {
      lang: 'en',
    });
  });

  it('rejects drivers that are not assigned to the delivery', async () => {
    authorization.canSendLocationUpdate.mockResolvedValue(false);

    await expect(service.handle(socket(), payload())).rejects.toMatchObject({
      code: WsErrorCode.FORBIDDEN,
    });
    expect(i18n.t).toHaveBeenCalledWith('messages.ws.notAssignedDriver', {
      lang: 'en',
    });
    expect(metrics.locationUpdates.inc).toHaveBeenCalledWith({
      result: 'rejected_unauthorized',
    });
    expect(natsPublisher.publish).not.toHaveBeenCalled();
  });

  it('sends a LOCATION_UPDATE_REJECTED notice when rate limited', async () => {
    throttler.allow.mockResolvedValue(false);

    await service.handle(socket(), payload(), 'req-1');

    expect(metrics.locationUpdates.inc).toHaveBeenCalledWith({
      result: 'rejected_rate_limited',
    });
    expect(writer.send).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        requestId: 'req-1',
        type: ServerMessageType.LOCATION_UPDATE_REJECTED,
        data: {
          deliveryId: 'delivery_1234',
          reason: WsErrorCode.RATE_LIMITED,
        },
      }),
      MessagePriority.NORMAL,
    );
    expect(natsPublisher.publish).not.toHaveBeenCalled();
  });

  it('stores the location and publishes to NATS on success', async () => {
    const p = payload();

    await service.handle(socket(), p, 'req-2');

    expect(redis.geoAdd).toHaveBeenCalledWith(
      'drivers:locations',
      31.2,
      30.5,
      'driver-1',
    );
    expect(redis.set).toHaveBeenCalledWith(
      'driver:location:driver-1',
      expect.objectContaining({
        lat: 30.5,
        lng: 31.2,
        accuracy: 4,
        speed: 10,
        heading: 45,
        deliveryId: 'delivery_1234',
      }),
      120,
    );
    expect(natsPublisher.publish).toHaveBeenCalledWith(
      RealtimeNatsSubjects.DELIVERY_LOCATION_UPDATED,
      expect.objectContaining({
        deliveryId: 'delivery_1234',
        driverId: 'driver-1',
        lat: 30.5,
        lng: 31.2,
      }),
    );
    expect(metrics.locationUpdates.inc).toHaveBeenCalledWith({
      result: 'accepted',
    });
  });

  it('continues the pipeline when redis writes fail', async () => {
    redis.geoAdd.mockRejectedValue(new Error('redis down'));
    redis.set.mockRejectedValue(new Error('redis down'));

    await expect(service.handle(socket(), payload())).resolves.toBeUndefined();
    expect(natsPublisher.publish).toHaveBeenCalled();
    expect(metrics.locationUpdates.inc).toHaveBeenCalledWith({
      result: 'accepted',
    });
  });

  it('authorizes using the validated delivery id', async () => {
    await service.handle(socket(), payload());
    expect(authorization.canSendLocationUpdate).toHaveBeenCalledWith(
      'driver-1',
      'delivery_1234',
    );
  });

  it('falls back to a generic message when validation fails without errors', async () => {
    jest
      .spyOn(validator, 'validate')
      .mockReturnValue({ valid: false } as ReturnType<
        LocationValidator['validate']
      >);

    await expect(service.handle(socket(), payload())).rejects.toMatchObject({
      code: WsErrorCode.INVALID_MESSAGE,
      message: 'Invalid location payload',
    });
    expect(authorization.canSendLocationUpdate).not.toHaveBeenCalled();
  });

  it('falls back to the translation key when i18n returns an empty string', async () => {
    i18n.t.mockReturnValue('');

    await expect(
      service.handle(socket(Role.USER), payload()),
    ).rejects.toMatchObject({
      code: WsErrorCode.FORBIDDEN,
      message: 'messages.ws.unauthorized',
    });
  });
});
