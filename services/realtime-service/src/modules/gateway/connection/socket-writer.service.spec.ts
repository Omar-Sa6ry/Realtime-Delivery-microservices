import { SocketWriter } from './socket-writer.service';
import { ConnectionRegistry } from './connection.registry';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import {
  MessagePriority,
  ServerMessageType,
  ServerMessage,
} from '@delivery-micro/shard';
import { TIMINGS } from '../../../common/common-types/constants';

describe('SocketWriter', () => {
  let writer: SocketWriter;
  let registry: ConnectionRegistry;
  let metrics: {
    messagesSent: { inc: jest.Mock };
    messagesDropped: { inc: jest.Mock };
    backpressureSaturated: { inc: jest.Mock };
  };

  const OPEN = 1;

  const makeSocket = (socketId = 's1') =>
    ({
      readyState: OPEN,
      OPEN,
      bufferedAmount: 0,
      send: jest.fn(),
      close: jest.fn(),
      data: { socketId },
    }) as any;

  const msg = (
    type: ServerMessageType = ServerMessageType.DELIVERY_STATUS_UPDATED,
    data: Record<string, unknown> = { deliveryId: 'd1' },
  ): ServerMessage => ({ type, data }) as ServerMessage;

  beforeEach(() => {
    jest.useFakeTimers();
    registry = new ConnectionRegistry();
    metrics = {
      messagesSent: { inc: jest.fn() },
      messagesDropped: { inc: jest.fn() },
      backpressureSaturated: { inc: jest.fn() },
    };
    writer = new SocketWriter(
      registry,
      metrics as unknown as RealtimeMetricsService,
    );
  });

  afterEach(() => {
    writer.onModuleDestroy();
    jest.useRealTimers();
  });

  describe('send (immediate priorities)', () => {
    it('serializes and sends NORMAL messages', () => {
      const socket = makeSocket();
      writer.send(socket, msg(), MessagePriority.NORMAL);

      expect(socket.send).toHaveBeenCalledWith(JSON.stringify(msg()));
      expect(metrics.messagesSent.inc).toHaveBeenCalledWith({
        type: ServerMessageType.DELIVERY_STATUS_UPDATED,
        priority: MessagePriority.NORMAL,
      });
    });

    it('skips sockets that are not open', () => {
      const socket = makeSocket();
      socket.readyState = 3;
      writer.send(socket, msg());

      expect(socket.send).not.toHaveBeenCalled();
      expect(metrics.messagesSent.inc).not.toHaveBeenCalled();
    });

    it('terminates slow consumers whose buffer is saturated', () => {
      const socket = makeSocket();
      socket.bufferedAmount = TIMINGS.MAX_BACKLOG * 1024 + 1;

      writer.send(socket, msg(), MessagePriority.CRITICAL);

      expect(metrics.backpressureSaturated.inc).toHaveBeenCalled();
      expect(metrics.messagesDropped.inc).toHaveBeenCalledWith(
        expect.objectContaining({ reason: 'slow_consumer' }),
      );
      expect(socket.close).toHaveBeenCalledWith(1013, 'Try again later');
      expect(socket.send).not.toHaveBeenCalled();
    });

    it('drops the message when send throws and records the metric', () => {
      const socket = makeSocket();
      socket.send.mockImplementation(() => {
        throw new Error('closed');
      });

      writer.send(socket, msg());

      expect(metrics.messagesDropped.inc).toHaveBeenCalledWith(
        expect.objectContaining({ reason: 'send_error' }),
      );
    });
  });

  describe('HIGH_FREQUENCY_LOSSY coalescing', () => {
    it('coalesces lossy messages per delivery and flushes the latest', () => {
      const socket = makeSocket();
      registry.register(socket);

      writer.send(
        socket,
        msg(ServerMessageType.DELIVERY_LOCATION_UPDATED, {
          deliveryId: 'd1',
          lat: 1,
        }),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );
      writer.send(
        socket,
        msg(ServerMessageType.DELIVERY_LOCATION_UPDATED, {
          deliveryId: 'd1',
          lat: 2,
        }),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );

      expect(socket.send).not.toHaveBeenCalled();

      jest.advanceTimersByTime(TIMINGS.LOCATION_COALESCE_MS);

      expect(socket.send).toHaveBeenCalledTimes(1);
      expect(socket.send).toHaveBeenCalledWith(
        expect.stringContaining('"lat":2'),
      );
    });

    it('drops pending lossy queues for sockets no longer in the registry', () => {
      const socket = makeSocket('gone');
      writer.send(
        socket,
        msg(ServerMessageType.DELIVERY_LOCATION_UPDATED),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );

      jest.advanceTimersByTime(TIMINGS.LOCATION_COALESCE_MS);

      expect(socket.send).not.toHaveBeenCalled();
    });

    it('drops pending lossy queues for closed sockets', () => {
      const socket = makeSocket('closed');
      registry.register(socket);
      writer.send(
        socket,
        msg(ServerMessageType.DELIVERY_LOCATION_UPDATED),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );
      socket.readyState = 3;

      jest.advanceTimersByTime(TIMINGS.LOCATION_COALESCE_MS);

      expect(socket.send).not.toHaveBeenCalled();
    });

    it('records dropped metrics when a lossy flush send fails', () => {
      const socket = makeSocket();
      registry.register(socket);
      socket.send.mockImplementation(() => {
        throw new Error('boom');
      });

      writer.send(
        socket,
        msg(ServerMessageType.DELIVERY_LOCATION_UPDATED),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );
      jest.advanceTimersByTime(TIMINGS.LOCATION_COALESCE_MS);

      expect(metrics.messagesDropped.inc).toHaveBeenCalledWith(
        expect.objectContaining({ reason: 'send_error' }),
      );
    });

    it('skips already-flushed (empty) queues on subsequent timer ticks', () => {
      const socket = makeSocket('flushed');
      registry.register(socket);

      writer.send(
        socket,
        msg(ServerMessageType.DELIVERY_LOCATION_UPDATED),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );

      jest.advanceTimersByTime(TIMINGS.LOCATION_COALESCE_MS);
      expect(socket.send).toHaveBeenCalledTimes(1);

      jest.advanceTimersByTime(TIMINGS.LOCATION_COALESCE_MS);
      expect(socket.send).toHaveBeenCalledTimes(1);
    });

    it('uses the message type as the coalescing key when no deliveryId is present', () => {
      const socket = makeSocket();
      registry.register(socket);

      writer.send(
        socket,
        msg(ServerMessageType.MEDIA_UPLOAD_PROGRESS, { userId: 'u1' }),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );
      writer.send(
        socket,
        msg(ServerMessageType.MEDIA_UPLOAD_PROGRESS, { userId: 'u1' }),
        MessagePriority.HIGH_FREQUENCY_LOSSY,
      );

      jest.advanceTimersByTime(TIMINGS.LOCATION_COALESCE_MS);
      expect(socket.send).toHaveBeenCalledTimes(1);
    });
  });

  describe('sendMany', () => {
    it('sends to every socket in the list', () => {
      const a = makeSocket('a');
      const b = makeSocket('b');
      writer.sendMany([a, b], msg(), MessagePriority.NORMAL);

      expect(a.send).toHaveBeenCalledTimes(1);
      expect(b.send).toHaveBeenCalledTimes(1);
    });

    it('handles an empty list', () => {
      expect(() => writer.sendMany([], msg())).not.toThrow();
    });
  });

  it('onModuleDestroy stops the flush timer', () => {
    const socket = makeSocket();
    registry.register(socket);
    writer.send(
      socket,
      msg(ServerMessageType.DELIVERY_LOCATION_UPDATED),
      MessagePriority.HIGH_FREQUENCY_LOSSY,
    );

    writer.onModuleDestroy();
    jest.advanceTimersByTime(TIMINGS.LOCATION_COALESCE_MS * 4);

    expect(socket.send).not.toHaveBeenCalled();
  });
});
