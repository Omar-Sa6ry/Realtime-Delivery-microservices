import {
  ClientMessageType,
  MessagePriority,
  REALTIME_MESSAGE_VERSION,
  ServerMessageType,
  ClientMessage,
  RealtimeMessage,
  ServerMessage,
} from './realtime-message';

describe('realtime-message', () => {
  describe('REALTIME_MESSAGE_VERSION', () => {
    it('is pinned to 1', () => {
      expect(REALTIME_MESSAGE_VERSION).toBe(1);
    });
  });

  describe('ClientMessageType', () => {
    it('exposes every client message type', () => {
      expect(Object.values(ClientMessageType)).toEqual([
        'PING',
        'SUBSCRIBE_DELIVERY',
        'UNSUBSCRIBE_DELIVERY',
        'LOCATION_UPDATE',
        'ACCEPT_ASSIGNMENT',
        'REJECT_ASSIGNMENT',
        'COMPLETE_DELIVERY',
        'ACK',
      ]);
    });

    it('maps members to their own names', () => {
      expect(ClientMessageType.PING).toBe('PING');
      expect(ClientMessageType.LOCATION_UPDATE).toBe('LOCATION_UPDATE');
      expect(ClientMessageType.ACCEPT_ASSIGNMENT).toBe('ACCEPT_ASSIGNMENT');
      expect(ClientMessageType.ACK).toBe('ACK');
    });
  });

  describe('ServerMessageType', () => {
    it('exposes every server message type', () => {
      expect(Object.values(ServerMessageType)).toEqual([
        'CONNECTED',
        'SUBSCRIBED',
        'UNSUBSCRIBED',
        'PONG',
        'ACK',
        'DELIVERY_LOCATION_UPDATED',
        'DELIVERY_STATUS_UPDATED',
        'DRIVER_ASSIGNED',
        'DRIVER_PRESENCE_UPDATED',
        'DELIVERY_COMPLETED',
        'DELIVERY_CANCELLED',
        'PAYMENT_STATUS_CHANGED',
        'NOTIFICATION_RECEIVED',
        'LOCATION_UPDATE_REJECTED',
        'ERROR',
        'ASSIGNMENT_OFFERED',
        'DRIVER_SEARCH_RETRY',
        'ASSIGNMENT_EXPIRED',
        'ASSIGNMENT_REJECTED',
        'MEDIA_UPLOAD_PROGRESS',
        'MEDIA_PROCESSING_PROGRESS',
        'MEDIA_READY',
        'MEDIA_DELETED',
        'MEDIA_FAILED',
      ]);
    });

    it('keeps assignment and media types distinct', () => {
      expect(ServerMessageType.ASSIGNMENT_OFFERED).toBe('ASSIGNMENT_OFFERED');
      expect(ServerMessageType.MEDIA_READY).toBe('MEDIA_READY');
      expect(ServerMessageType.ERROR).toBe('ERROR');
      expect(
        new Set(Object.values(ServerMessageType)).size,
      ).toBe(Object.values(ServerMessageType).length);
    });
  });

  describe('MessagePriority', () => {
    it('exposes the three priority levels', () => {
      expect(Object.values(MessagePriority)).toEqual([
        'CRITICAL',
        'NORMAL',
        'HIGH_FREQUENCY_LOSSY',
      ]);
    });
  });

  describe('message envelopes', () => {
    it('builds a full RealtimeMessage envelope', () => {
      const envelope: RealtimeMessage<{ deliveryId: string }> = {
        messageId: 'msg-1',
        type: ServerMessageType.DELIVERY_LOCATION_UPDATED,
        version: REALTIME_MESSAGE_VERSION,
        timestamp: new Date(1700000000000).toISOString(),
        priority: MessagePriority.HIGH_FREQUENCY_LOSSY,
        data: { deliveryId: 'd-1' },
      };

      expect(envelope).toEqual({
        messageId: 'msg-1',
        type: 'DELIVERY_LOCATION_UPDATED',
        version: 1,
        timestamp: '2023-11-14T22:13:20.000Z',
        priority: 'HIGH_FREQUENCY_LOSSY',
        data: { deliveryId: 'd-1' },
      });
      expect(envelope.timestamp).toEqual(
        expect.stringMatching(/^\d{4}-\d{2}-\d{2}T/),
      );
    });

    it('builds a ClientMessage with an optional requestId', () => {
      const withRequest: ClientMessage<{ lat: number }> = {
        requestId: 'r-1',
        type: ClientMessageType.LOCATION_UPDATE,
        data: { lat: 1 },
      };
      const withoutRequest: ClientMessage<void> = {
        type: ClientMessageType.PING,
        data: undefined,
      };

      expect(withRequest.requestId).toBe('r-1');
      expect('requestId' in withoutRequest).toBe(false);
      expect(withoutRequest.type).toBe(ClientMessageType.PING);
    });

    it('builds a ServerMessage with an optional requestId', () => {
      const message: ServerMessage<{ status: string }> = {
        requestId: 'r-2',
        type: ServerMessageType.DELIVERY_STATUS_UPDATED,
        data: { status: 'PICKED_UP' },
      };
      const noRequest: ServerMessage = {
        type: ServerMessageType.CONNECTED,
        data: null,
      };

      expect(message).toEqual({
        requestId: 'r-2',
        type: 'DELIVERY_STATUS_UPDATED',
        data: { status: 'PICKED_UP' },
      });
      expect(noRequest.requestId).toBeUndefined();
      expect(noRequest.type).toBe('CONNECTED');
    });

    it('serializes envelopes to JSON without losing type information', () => {
      const envelope: RealtimeMessage<number> = {
        messageId: 'msg-2',
        type: ServerMessageType.PONG,
        version: REALTIME_MESSAGE_VERSION,
        timestamp: new Date().toISOString(),
        priority: MessagePriority.CRITICAL,
        data: 42,
      };

      expect(JSON.parse(JSON.stringify(envelope))).toEqual(envelope);
    });
  });
});
