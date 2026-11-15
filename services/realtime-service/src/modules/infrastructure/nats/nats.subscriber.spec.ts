import { NatsSubscriber } from './nats.subscriber';
import { RealtimeNatsService } from './nats.service';
import { SubscriptionStore } from '../../features/subscription/subscription.store';
import { ConnectionService } from '../../gateway/connection/connection.service';
import { SocketWriter } from '../../gateway/connection/socket-writer.service';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import {
  MessagePriority,
  RealtimeNatsSubjects,
  Role,
  ServerMessageType,
} from '@delivery-micro/shard';

describe('NatsSubscriber', () => {
  let subscriber: NatsSubscriber;
  let nats: {
    getClient: jest.Mock;
    getCodec: jest.Mock;
    onModuleDestroy: jest.Mock;
  };
  let subscriptionStore: { getDeliverySubscribers: jest.Mock };
  let connectionService: {
    getLocalSocketsByUser: jest.Mock;
    getLocalSocketsByRole: jest.Mock;
    getLocalConnection: jest.Mock;
  };
  let writer: { sendMany: jest.Mock };
  let metrics: Record<string, never>;
  let client: { subscribe: jest.Mock };
  let codec: { decode: jest.Mock };

  const decodeAndHandle = async (subject: string, message: any) => {
    codec.decode.mockReturnValue(message);
    await (subscriber as any).handleMessage(subject, new Uint8Array([1, 2, 3]));
  };

  beforeEach(() => {
    client = { subscribe: jest.fn() };
    codec = { decode: jest.fn() };
    nats = {
      getClient: jest.fn().mockReturnValue(client),
      getCodec: jest.fn().mockReturnValue(codec),
      onModuleDestroy: jest.fn().mockResolvedValue(undefined),
    };
    subscriptionStore = {
      getDeliverySubscribers: jest.fn().mockResolvedValue([]),
    };
    connectionService = {
      getLocalSocketsByUser: jest.fn().mockReturnValue([]),
      getLocalSocketsByRole: jest.fn().mockReturnValue([]),
      getLocalConnection: jest.fn().mockReturnValue(undefined),
    };
    writer = { sendMany: jest.fn() };
    metrics = {};

    subscriber = new NatsSubscriber(
      nats as unknown as RealtimeNatsService,
      subscriptionStore as unknown as SubscriptionStore,
      connectionService as unknown as ConnectionService,
      writer as unknown as SocketWriter,
      metrics as unknown as RealtimeMetricsService,
    );
  });

  describe('subscribe lifecycle', () => {
    it('subscribes to all realtime subjects on init', async () => {
      await subscriber.onModuleInit();

      expect(client.subscribe).toHaveBeenCalledWith(
        RealtimeNatsSubjects.DELIVERY_LOCATION_UPDATED,
      );
      expect(client.subscribe).toHaveBeenCalledWith(
        RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED,
      );
      expect(client.subscribe).toHaveBeenCalledWith(
        RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED,
      );
      expect(client.subscribe).toHaveBeenCalledWith('payment.status.updated');
      expect(client.subscribe).toHaveBeenCalledWith('notification.user.*');
      expect(client.subscribe).toHaveBeenCalledTimes(12);
    });

    it('retries subscription when the NATS client is not ready yet', async () => {
      jest.useFakeTimers();
      nats.getClient.mockReturnValueOnce(null).mockReturnValue(client);

      await subscriber.onModuleInit();
      expect(client.subscribe).not.toHaveBeenCalled();

      await jest.advanceTimersByTimeAsync(2000);
      expect(client.subscribe).toHaveBeenCalled();
      jest.useRealTimers();
    });

    it('destroys the shared NATS connection on shutdown', async () => {
      await subscriber.onModuleDestroy();
      expect(nats.onModuleDestroy).toHaveBeenCalled();
    });
  });

  describe('handleMessage routing', () => {
    it('routes assignment offers directly to the driver sockets', async () => {
      const sockets = [{ id: 's1' }];
      connectionService.getLocalSocketsByUser.mockReturnValue(sockets);

      await decodeAndHandle(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED, {
        type: ServerMessageType.ASSIGNMENT_OFFERED,
        priority: MessagePriority.CRITICAL,
        data: { driverId: 'drv1', deliveryId: 'd1' },
      });

      expect(connectionService.getLocalSocketsByUser).toHaveBeenCalledWith(
        'drv1',
      );
      expect(writer.sendMany).toHaveBeenCalledWith(
        sockets,
        expect.objectContaining({ type: ServerMessageType.ASSIGNMENT_OFFERED }),
        MessagePriority.CRITICAL,
      );
    });

    it('ignores assignment offers without a driverId', async () => {
      await decodeAndHandle(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED, {
        data: {},
      });
      expect(writer.sendMany).not.toHaveBeenCalled();
    });

    it('routes user notifications from the notification subject', async () => {
      const sockets = [{ id: 's1' }];
      connectionService.getLocalSocketsByUser.mockReturnValue(sockets);

      await decodeAndHandle(`${'notification.user'}.u1`, {
        data: { title: 'hi' },
      });

      expect(connectionService.getLocalSocketsByUser).toHaveBeenCalledWith(
        'u1',
      );
      expect(writer.sendMany).toHaveBeenCalledWith(
        sockets,
        expect.objectContaining({
          type: ServerMessageType.NOTIFICATION_RECEIVED,
          data: { title: 'hi' },
        }),
        MessagePriority.NORMAL,
      );
    });

    it('routes payment status updates to the user', async () => {
      const sockets = [{ id: 's1' }];
      connectionService.getLocalSocketsByUser.mockReturnValue(sockets);

      await decodeAndHandle('payment.status.updated', {
        userId: 'u1',
        status: 'COMPLETED',
      });

      expect(writer.sendMany).toHaveBeenCalledWith(
        sockets,
        expect.objectContaining({
          type: ServerMessageType.PAYMENT_STATUS_CHANGED,
        }),
        MessagePriority.NORMAL,
      );
    });

    it('skips payment updates without a userId', async () => {
      await decodeAndHandle('payment.status.updated', { status: 'FAILED' });
      expect(writer.sendMany).not.toHaveBeenCalled();
    });

    it('routes presence updates to admins', async () => {
      const admins = [{ id: 'a1' }];
      connectionService.getLocalSocketsByRole.mockReturnValue(admins);

      await decodeAndHandle(RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED, {
        type: ServerMessageType.DRIVER_PRESENCE_UPDATED,
        data: { driverId: 'drv1', status: 'ONLINE' },
      });

      expect(connectionService.getLocalSocketsByRole).toHaveBeenCalledWith(
        Role.ADMIN,
      );
      expect(writer.sendMany).toHaveBeenCalledWith(
        admins,
        expect.objectContaining({
          type: ServerMessageType.DRIVER_PRESENCE_UPDATED,
        }),
      );
    });

    it('routes media events to the target user', async () => {
      const sockets = [{ id: 's1' }];
      connectionService.getLocalSocketsByUser.mockReturnValue(sockets);

      await decodeAndHandle(RealtimeNatsSubjects.MEDIA_UPLOAD_PROGRESS, {
        type: ServerMessageType.MEDIA_UPLOAD_PROGRESS,
        priority: MessagePriority.HIGH_FREQUENCY_LOSSY,
        data: { userId: 'u1', progress: 40 },
      });

      expect(writer.sendMany).toHaveBeenCalledWith(
        sockets,
        expect.objectContaining({
          type: ServerMessageType.MEDIA_UPLOAD_PROGRESS,
        }),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );
    });

    it('skips media events without a userId', async () => {
      await decodeAndHandle(RealtimeNatsSubjects.MEDIA_READY, { data: {} });
      expect(writer.sendMany).not.toHaveBeenCalled();
    });

    it('fans delivery events out to subscribed sockets', async () => {
      const socketA = { id: 'a' };
      subscriptionStore.getDeliverySubscribers.mockResolvedValue(['a']);
      connectionService.getLocalConnection.mockImplementation((id: string) =>
        id === 'a' ? socketA : undefined,
      );

      await decodeAndHandle(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, {
        type: ServerMessageType.DELIVERY_STATUS_UPDATED,
        data: { deliveryId: 'd1' },
      });

      expect(subscriptionStore.getDeliverySubscribers).toHaveBeenCalledWith(
        'd1',
      );
      expect(writer.sendMany).toHaveBeenCalledWith(
        [socketA],
        expect.objectContaining({
          type: ServerMessageType.DELIVERY_STATUS_UPDATED,
        }),
        MessagePriority.NORMAL,
      );
    });

    it('also includes the customerId sockets when present', async () => {
      const subscribed = { id: 'a' };
      const customerSocket = { id: 'c' };
      subscriptionStore.getDeliverySubscribers.mockResolvedValue(['a']);
      connectionService.getLocalConnection.mockReturnValue(subscribed);
      connectionService.getLocalSocketsByUser.mockReturnValue([customerSocket]);

      await decodeAndHandle(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, {
        type: ServerMessageType.DELIVERY_STATUS_UPDATED,
        data: { deliveryId: 'd1', customerId: 'cust1' },
      });

      expect(writer.sendMany).toHaveBeenCalledWith(
        [subscribed, customerSocket],
        expect.anything(),
        MessagePriority.NORMAL,
      );
    });

    it('does not duplicate sockets already in the delivery room', async () => {
      const socket = { id: 'a' };
      subscriptionStore.getDeliverySubscribers.mockResolvedValue(['a']);
      connectionService.getLocalConnection.mockReturnValue(socket);
      connectionService.getLocalSocketsByUser.mockReturnValue([socket]);

      await decodeAndHandle(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, {
        type: ServerMessageType.DELIVERY_STATUS_UPDATED,
        data: { deliveryId: 'd1', customerId: 'cust1' },
      });

      expect(writer.sendMany).toHaveBeenCalledWith(
        [socket],
        expect.anything(),
        expect.anything(),
      );
    });

    it('marks location fan-outs as HIGH_FREQUENCY_LOSSY by default', async () => {
      subscriptionStore.getDeliverySubscribers.mockResolvedValue(['a']);
      connectionService.getLocalConnection.mockReturnValue({ id: 'a' });

      await decodeAndHandle(RealtimeNatsSubjects.DELIVERY_LOCATION_UPDATED, {
        type: ServerMessageType.DELIVERY_LOCATION_UPDATED,
        data: { deliveryId: 'd1', lat: 1, lng: 2 },
      });

      expect(writer.sendMany).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );
    });

    it('prefers an explicit message priority over the default', async () => {
      subscriptionStore.getDeliverySubscribers.mockResolvedValue(['a']);
      connectionService.getLocalConnection.mockReturnValue({ id: 'a' });

      await decodeAndHandle(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, {
        type: ServerMessageType.DELIVERY_STATUS_UPDATED,
        priority: MessagePriority.CRITICAL,
        data: { deliveryId: 'd1' },
      });

      expect(writer.sendMany).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        MessagePriority.CRITICAL,
      );
    });

    it('skips delivery fan-out when deliveryId is missing', async () => {
      await decodeAndHandle(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, {
        data: {},
      });
      expect(writer.sendMany).not.toHaveBeenCalled();
    });

    it('ignores messages that fail to decode', async () => {
      codec.decode.mockImplementation(() => {
        throw new Error('bad json');
      });
      const warn = jest.spyOn((subscriber as any).logger, 'warn');

      await (subscriber as any).handleMessage(
        RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
        new Uint8Array([9]),
      );

      expect(warn).toHaveBeenCalled();
      expect(writer.sendMany).not.toHaveBeenCalled();
    });

    it('returns early from every user fan-out when no local sockets exist', async () => {
      connectionService.getLocalSocketsByUser.mockReturnValue([]);

      await decodeAndHandle(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED, {
        data: { driverId: 'drv1' },
      });
      await decodeAndHandle('notification.user.u1', { data: { title: 'hi' } });
      await decodeAndHandle('payment.status.updated', { userId: 'u1' });
      await decodeAndHandle(RealtimeNatsSubjects.MEDIA_UPLOAD_PROGRESS, {
        type: ServerMessageType.MEDIA_UPLOAD_PROGRESS,
        data: { userId: 'u1' },
      });

      expect(connectionService.getLocalSocketsByUser).toHaveBeenCalledTimes(4);
      expect(writer.sendMany).not.toHaveBeenCalled();
    });

    it('ignores notification subjects that end with an empty user id', async () => {
      await decodeAndHandle('notification.user.', { data: { title: 'hi' } });

      expect(connectionService.getLocalSocketsByUser).not.toHaveBeenCalled();
      expect(writer.sendMany).not.toHaveBeenCalled();
    });

    it('defaults assignment offer type and priority when omitted', async () => {
      const sockets = [{ id: 's1' }];
      connectionService.getLocalSocketsByUser.mockReturnValue(sockets);

      await decodeAndHandle(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED, {
        data: { driverId: 'drv1', deliveryId: 'd1' },
      });

      expect(writer.sendMany).toHaveBeenCalledWith(
        sockets,
        {
          type: ServerMessageType.ASSIGNMENT_OFFERED,
          data: { driverId: 'drv1', deliveryId: 'd1' },
        },
        MessagePriority.CRITICAL,
      );
    });

    it('defaults media fan-outs to NORMAL priority when omitted', async () => {
      const sockets = [{ id: 's1' }];
      connectionService.getLocalSocketsByUser.mockReturnValue(sockets);

      await decodeAndHandle(RealtimeNatsSubjects.MEDIA_READY, {
        type: ServerMessageType.MEDIA_READY,
        data: { userId: 'u1' },
      });

      expect(writer.sendMany).toHaveBeenCalledWith(
        sockets,
        expect.objectContaining({ type: ServerMessageType.MEDIA_READY }),
        MessagePriority.NORMAL,
      );
    });
  });

  describe('consume', () => {
    it('logs handler errors without killing the subscription', async () => {
      const errorLog = jest.spyOn((subscriber as any).logger, 'error');
      const sub = {
        async *[Symbol.asyncIterator]() {
          yield { data: new Uint8Array([1]) };
          yield { data: new Uint8Array([2]) };
        },
      };
      // Decode succeeds but the fan-out throws -> consume catches and keeps looping
      codec.decode.mockReturnValue({
        type: ServerMessageType.DELIVERY_STATUS_UPDATED,
        data: { deliveryId: 'd1' },
      });
      subscriptionStore.getDeliverySubscribers.mockRejectedValue(
        new Error('redis down'),
      );

      await (subscriber as any).consume('subject', sub);

      expect(errorLog).toHaveBeenCalledTimes(2);
    });

    it('logs when the subscription iterator itself fails', async () => {
      const errorLog = jest.spyOn((subscriber as any).logger, 'error');
      const sub = {
        async *[Symbol.asyncIterator]() {
          yield* [];
          throw new Error('subscription closed');
        },
      };

      await (subscriber as any).consume('subject', sub);
      expect(errorLog).toHaveBeenCalledWith(
        expect.stringContaining('terminated'),
      );
    });
  });
});
