import {
  buildServerMessage,
  VALIDATION,
  RATE_ACTIONS,
} from './websocket.types';
import {
  ClientMessageType,
  MessagePriority,
  ServerMessageType,
  WsErrorCode,
} from '@delivery-micro/shard';

describe('buildServerMessage', () => {
  it('builds an outbound message with timestamp', () => {
    const message = buildServerMessage(
      ServerMessageType.SUBSCRIBED,
      { deliveryId: 'd1' },
      'req-1',
      MessagePriority.NORMAL,
    );

    expect(message.type).toBe(ServerMessageType.SUBSCRIBED);
    expect(message.data).toEqual({ deliveryId: 'd1' });
    expect(message.requestId).toBe('req-1');
    expect(message.priority).toBe(MessagePriority.NORMAL);
    expect(typeof message.timestamp).toBe('string');
    expect(Number.isNaN(Date.parse(message.timestamp))).toBe(false);
  });

  it('works without requestId and priority', () => {
    const message = buildServerMessage(ServerMessageType.PONG, { ts: 1 });
    expect(message.requestId).toBeUndefined();
    expect(message.priority).toBeUndefined();
  });
});

describe('VALIDATION', () => {
  it('requires deliveryId for subscribe/unsubscribe', () => {
    for (const type of [
      ClientMessageType.SUBSCRIBE_DELIVERY,
      ClientMessageType.UNSUBSCRIBE_DELIVERY,
    ]) {
      const validate = VALIDATION[type]!;
      expect(() => validate({ deliveryId: 'd1' })).not.toThrow();
      expect(() => validate({})).toThrow();
      expect(() => validate(undefined)).toThrow();
      expect(() => validate({ deliveryId: 123 })).toThrow();
    }
  });

  it('requires deliveryId and commandId for commands', () => {
    for (const type of [
      ClientMessageType.ACCEPT_ASSIGNMENT,
      ClientMessageType.REJECT_ASSIGNMENT,
      ClientMessageType.COMPLETE_DELIVERY,
    ]) {
      const validate = VALIDATION[type]!;
      expect(() =>
        validate({ deliveryId: 'd1', commandId: 'c1' }),
      ).not.toThrow();
      expect(() => validate({ deliveryId: 'd1' })).toThrow();
      expect(() => validate({ commandId: 'c1' })).toThrow();
      expect(() => validate({ deliveryId: 'd1', commandId: 5 })).toThrow();
    }
  });

  it('throws WsException with INVALID_MESSAGE code', () => {
    try {
      VALIDATION[ClientMessageType.SUBSCRIBE_DELIVERY]!({});
      fail('expected throw');
    } catch (err: any) {
      expect(err.code).toBe(WsErrorCode.INVALID_MESSAGE);
      expect(err.message).toBe('deliveryId required');
    }
  });

  it('has no validators for ping/ack/location types', () => {
    expect(VALIDATION[ClientMessageType.PING]).toBeUndefined();
    expect(VALIDATION[ClientMessageType.ACK]).toBeUndefined();
    expect(VALIDATION[ClientMessageType.LOCATION_UPDATE]).toBeUndefined();
  });
});

describe('RATE_ACTIONS', () => {
  it('maps subscribe-family messages to subscribe action', () => {
    expect(RATE_ACTIONS[ClientMessageType.SUBSCRIBE_DELIVERY]).toBe(
      'subscribe',
    );
    expect(RATE_ACTIONS[ClientMessageType.UNSUBSCRIBE_DELIVERY]).toBe(
      'subscribe',
    );
  });

  it('maps command-family messages to command action', () => {
    expect(RATE_ACTIONS[ClientMessageType.ACCEPT_ASSIGNMENT]).toBe('command');
    expect(RATE_ACTIONS[ClientMessageType.REJECT_ASSIGNMENT]).toBe('command');
    expect(RATE_ACTIONS[ClientMessageType.COMPLETE_DELIVERY]).toBe('command');
  });

  it('does not rate-limit ping/ack/location through the chain', () => {
    expect(RATE_ACTIONS[ClientMessageType.PING]).toBeUndefined();
    expect(RATE_ACTIONS[ClientMessageType.ACK]).toBeUndefined();
    expect(RATE_ACTIONS[ClientMessageType.LOCATION_UPDATE]).toBeUndefined();
  });
});
