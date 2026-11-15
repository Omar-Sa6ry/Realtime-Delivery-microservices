import {
  redisKeys,
  TTL,
  TIMINGS,
  RATE_LIMITS,
  SERVER_MESSAGE_PRIORITY,
} from './constants';
import { MessagePriority, ServerMessageType } from '@delivery-micro/shard';

describe('constants', () => {
  describe('redisKeys', () => {
    it('builds connection keys', () => {
      expect(redisKeys.connection('s1')).toBe('ws:connection:s1');
      expect(redisKeys.userSockets('u1')).toBe('ws:user:u1:sockets');
      expect(redisKeys.roleSockets('driver')).toBe('ws:role:driver:sockets');
      expect(redisKeys.nodeConnections('n1')).toBe('ws:node:n1:connections');
    });

    it('builds subscription keys', () => {
      expect(redisKeys.deliverySubscribers('d1')).toBe(
        'ws:delivery:d1:subscribers',
      );
      expect(redisKeys.socketSubscriptions('s1')).toBe(
        'ws:socket:s1:subscriptions',
      );
    });

    it('builds presence and rate keys', () => {
      expect(redisKeys.presence('u1')).toBe('ws:presence:u1');
      expect(redisKeys.rate('u1', 'command')).toBe('ws:rate:u1:command');
      expect(redisKeys.locationBucket('drv1')).toBe('ws:rate:drv1:location');
    });

    it('builds idempotency, dedup and authz keys', () => {
      expect(redisKeys.idempotency('c1')).toBe('realtime:idempotency:c1');
      expect(redisKeys.processed('e1')).toBe('realtime:processed:e1');
      expect(redisKeys.authzCache('u1', 'd1')).toBe('realtime:authz:u1:d1');
    });

    it('builds driver location keys', () => {
      expect(redisKeys.driverLocation('drv1')).toBe('driver:location:drv1');
      expect(redisKeys.driverLocationsIndex()).toBe('drivers:locations');
    });
  });

  describe('TTL', () => {
    it('derives heartbeat TTL in seconds from ms', () => {
      expect(TTL.HEARTBEAT).toBe(Math.floor(60000 / 1000));
      expect(TTL.CONNECTION).toBe(60);
      expect(TTL.DRIVER_LOCATION).toBe(120);
      expect(TTL.IDEMPOTENCY).toBe(24 * 60 * 60);
      expect(TTL.PROCESSED).toBe(24 * 60 * 60);
      expect(TTL.AUTHZ_CACHE).toBe(30);
    });
  });

  describe('TIMINGS', () => {
    it('exposes expected timing constants', () => {
      expect(TIMINGS.LOCATION_COALESCE_MS).toBe(250);
      expect(TIMINGS.GRPC_TIMEOUT_MS).toBe(2000);
      expect(TIMINGS.CIRCUIT_RESET_MS).toBe(10_000);
      expect(TIMINGS.CIRCUIT_FAILURE_THRESHOLD).toBe(5);
      expect(TIMINGS.LOCATION_MAX_AGE_MS).toBe(60_000);
      expect(TIMINGS.STALE_CRON_MS).toBe(20_000);
    });
  });

  describe('RATE_LIMITS', () => {
    it('exposes expected rate limits', () => {
      expect(RATE_LIMITS.CONNECT_PER_MINUTE).toBe(10);
      expect(RATE_LIMITS.COMMANDS_PER_MINUTE).toBe(30);
      expect(RATE_LIMITS.SUBSCRIBE_PER_MINUTE).toBe(60);
      expect(RATE_LIMITS.LOCATION_PER_SECOND).toBe(5);
      expect(RATE_LIMITS.LOCATION_BURST).toBe(10);
    });
  });

  describe('SERVER_MESSAGE_PRIORITY', () => {
    it('marks location updates as lossy', () => {
      expect(
        SERVER_MESSAGE_PRIORITY[ServerMessageType.DELIVERY_LOCATION_UPDATED],
      ).toBe(MessagePriority.HIGH_FREQUENCY_LOSSY);
    });

    it('marks critical lifecycle events as CRITICAL', () => {
      for (const type of [
        ServerMessageType.DRIVER_ASSIGNED,
        ServerMessageType.DELIVERY_COMPLETED,
        ServerMessageType.DELIVERY_CANCELLED,
        ServerMessageType.PAYMENT_STATUS_CHANGED,
      ]) {
        expect(SERVER_MESSAGE_PRIORITY[type]).toBe(MessagePriority.CRITICAL);
      }
    });

    it('marks status and presence events as NORMAL', () => {
      expect(
        SERVER_MESSAGE_PRIORITY[ServerMessageType.DELIVERY_STATUS_UPDATED],
      ).toBe(MessagePriority.NORMAL);
      expect(
        SERVER_MESSAGE_PRIORITY[ServerMessageType.DRIVER_PRESENCE_UPDATED],
      ).toBe(MessagePriority.NORMAL);
    });
  });
});
