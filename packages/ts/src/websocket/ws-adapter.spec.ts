import { EventEmitter } from 'events';
import { WsAdapter } from '@nestjs/platform-ws';
import { of } from 'rxjs';
import { RealtimeWsAdapter } from './ws-adapter';
import { WsErrorCode, WsException } from './ws-errors';

class FakeClient extends EventEmitter {
  readonly OPEN = 1;
  readyState = 1;
  readonly send = jest.fn();
}

type Handler = { message: string; callback: jest.Mock };

const flush = () => new Promise<void>((resolve) => setImmediate(resolve));

const json = (payload: unknown) => Buffer.from(JSON.stringify(payload));

const readEnvelope = (client: FakeClient) =>
  JSON.parse(client.send.mock.calls[0][0] as string);

const makeAdapter = (options?: { maxPayload?: number }) => {
  const adapter = new RealtimeWsAdapter({}, options);
  jest.spyOn(adapter.logger, 'warn').mockImplementation();
  jest.spyOn(adapter.logger, 'error').mockImplementation();
  return adapter;
};

const makeHandlers = (map: Record<string, (message: any) => any>): Handler[] =>
  Object.entries(map).map(([message, callback]) => ({
    message,
    callback: jest.fn(callback),
  }));

describe('RealtimeWsAdapter', () => {
  describe('maxPayload configuration', () => {
    it('prefers the explicit option', () => {
      expect(new RealtimeWsAdapter({}, { maxPayload: 4096 })['maxPayload']).toBe(
        4096,
      );
    });

    it('falls back to the WS_MAX_PAYLOAD env var', () => {
      process.env.WS_MAX_PAYLOAD = '8192';
      try {
        expect(new RealtimeWsAdapter({})['maxPayload']).toBe(8192);
      } finally {
        delete process.env.WS_MAX_PAYLOAD;
      }
    });

    it('falls back to 16384 when nothing is configured', () => {
      delete process.env.WS_MAX_PAYLOAD;
      expect(new RealtimeWsAdapter({})['maxPayload']).toBe(16384);
    });

    it('falls back to 16384 when the env var is not a number', () => {
      process.env.WS_MAX_PAYLOAD = 'not-a-number';
      try {
        expect(new RealtimeWsAdapter({})['maxPayload']).toBe(16384);
      } finally {
        delete process.env.WS_MAX_PAYLOAD;
      }
    });
  });

  describe('create', () => {
    it('merges maxPayload and disables per-message deflate', () => {
      const createSpy = jest
        .spyOn(WsAdapter.prototype, 'create')
        .mockReturnValue({ fake: 'server' });
      try {
        const adapter = new RealtimeWsAdapter({}, { maxPayload: 512 });

        const server = adapter.create(3000, { path: '/ws' });

        expect(createSpy).toHaveBeenCalledWith(3000, {
          path: '/ws',
          maxPayload: 512,
          perMessageDeflate: false,
        });
        expect(server).toEqual({ fake: 'server' });
      } finally {
        createSpy.mockRestore();
      }
    });

    it('keeps caller supplied options', () => {
      const createSpy = jest
        .spyOn(WsAdapter.prototype, 'create')
        .mockReturnValue(undefined);
      try {
        delete process.env.WS_MAX_PAYLOAD;
        const adapter = new RealtimeWsAdapter({});

        adapter.create(8080, { noServer: true, clientTracking: false });

        expect(createSpy).toHaveBeenCalledWith(8080, {
          noServer: true,
          clientTracking: false,
          maxPayload: 16384,
          perMessageDeflate: false,
        });
      } finally {
        createSpy.mockRestore();
      }
    });
  });

  describe('bindMessageHandlers routing', () => {
    it('invokes the matching handler and sends its result', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () => of({ pong: true }),
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit(
        'message',
        json({ type: 'PING', requestId: 'r-1', data: { ts: 1 } }),
      );

      expect(handler.callback).toHaveBeenCalledWith({
        type: 'PING',
        requestId: 'r-1',
        data: { ts: 1 },
      });
      expect(client.send).toHaveBeenCalledTimes(1);
      expect(client.send).toHaveBeenCalledWith('{"pong":true}');
      expect(adapter.logger.warn).not.toHaveBeenCalled();
    });

    it('supports the legacy event field as the message type', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({ PONG: () => of({ ok: 1 }) });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ event: 'PONG' }));

      expect(handler.callback).toHaveBeenCalledTimes(1);
      expect(client.send).toHaveBeenCalledWith('{"ok":1}');
    });

    it('unwraps a ws style payload envelope', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({ PING: () => of({ pong: true }) });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', { data: json({ type: 'PING' }) });

      expect(handler.callback).toHaveBeenCalledTimes(1);
      expect(client.send).toHaveBeenCalledTimes(1);
    });

    it('unwraps an array style payload', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({ PING: () => of({ pong: true }) });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', [json({ type: 'PING' })]);

      expect(handler.callback).toHaveBeenCalledTimes(1);
      expect(client.send).toHaveBeenCalledTimes(1);
    });

    it('dispatches each message type to its own handler', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const handlers = makeHandlers({
        PING: () => of({ kind: 'ping' }),
        ACK: () => of({ kind: 'ack' }),
      });

      adapter.bindMessageHandlers(client, handlers, {});
      client.emit('message', json({ type: 'ACK' }));
      client.emit('message', json({ type: 'PING' }));

      expect(handlers[1].callback).toHaveBeenCalledTimes(1);
      expect(handlers[0].callback).toHaveBeenCalledTimes(1);
      expect(client.send.mock.calls.map((c) => c[0])).toEqual([
        '{"kind":"ack"}',
        '{"kind":"ping"}',
      ]);
    });

    it('flattens nested observables returned by a handler', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () => of(of({ nested: true })),
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));

      expect(client.send).toHaveBeenCalledWith('{"nested":true}');
    });

    it('awaits promises returned by a handler', async () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: async () => ({ async: true }),
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));
      expect(client.send).not.toHaveBeenCalled();

      await flush();

      expect(handler.callback).toHaveBeenCalledTimes(1);
      expect(client.send).toHaveBeenCalledWith('{"async":true}');
    });

    it('does not answer messages whose result is nil', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({ PING: () => of(null) });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));

      expect(handler.callback).toHaveBeenCalledTimes(1);
      expect(client.send).not.toHaveBeenCalled();
    });

    it('stops reacting to messages after the socket closes', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({ PING: () => of({ pong: true }) });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));
      expect(client.send).toHaveBeenCalledTimes(1);

      client.emit('close');
      client.emit('message', json({ type: 'PING' }));

      expect(handler.callback).toHaveBeenCalledTimes(1);
      expect(client.send).toHaveBeenCalledTimes(1);
    });

    it('computes but drops responses when the socket is no longer open', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({ PING: () => of({ pong: true }) });

      adapter.bindMessageHandlers(client, [handler], {});
      client.readyState = 3;
      client.emit('message', json({ type: 'PING' }));

      expect(handler.callback).toHaveBeenCalledTimes(1);
      expect(client.send).not.toHaveBeenCalled();
    });
  });

  describe('bindMessageHandlers invalid input', () => {
    it('answers malformed JSON with an INVALID_MESSAGE envelope', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();

      adapter.bindMessageHandlers(client, [], {});
      client.emit('message', Buffer.from('{not json'));

      expect(readEnvelope(client)).toEqual({
        type: 'ERROR',
        timestamp: expect.any(String),
        data: {
          code: 'INVALID_MESSAGE',
          message: 'Invalid JSON message',
          retryable: false,
        },
      });
      expect(client.send).toHaveBeenCalledTimes(1);
    });

    it('answers an empty payload with an INVALID_MESSAGE envelope', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();

      adapter.bindMessageHandlers(client, [], {});
      client.emit('message');

      expect(readEnvelope(client).data.message).toBe('Invalid JSON message');
    });

    it('rejects a JSON null payload', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();

      adapter.bindMessageHandlers(client, [], {});
      client.emit('message', Buffer.from('null'));

      expect(readEnvelope(client).data.message).toBe(
        'Message must be an object',
      );
    });

    it('rejects a JSON scalar payload', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();

      adapter.bindMessageHandlers(client, [], {});
      client.emit('message', Buffer.from('"just a string"'));

      expect(readEnvelope(client).data.message).toBe(
        'Message must be an object',
      );
    });

    it('rejects a payload without a type', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();

      adapter.bindMessageHandlers(client, [], {});
      client.emit('message', json({ data: {} }));

      expect(readEnvelope(client).data.message).toBe('Missing message type');
    });

    it('rejects an unknown message type and echoes the requestId', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({ PING: () => of({ pong: true }) });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'TELEPORT', requestId: 'r-77' }));

      expect(handler.callback).not.toHaveBeenCalled();
      expect(adapter.logger.warn).toHaveBeenCalledWith(
        'Unknown WebSocket message type: TELEPORT',
      );
      expect(client.send).toHaveBeenCalledTimes(1);
      expect(readEnvelope(client)).toEqual({
        type: 'ERROR',
        timestamp: expect.any(String),
        requestId: 'r-77',
        data: {
          code: 'INVALID_MESSAGE',
          message: 'Unknown message type: TELEPORT',
          retryable: false,
        },
      });
    });
  });

  describe('bindMessageHandlers handler failures', () => {
    it('answers a WsException thrown by the handler', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () => {
          throw new WsException(
            WsErrorCode.RATE_LIMITED,
            'slow down',
            true,
          );
        },
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING', requestId: 'r-9' }));

      expect(client.send).toHaveBeenCalledTimes(1);
      expect(readEnvelope(client)).toEqual({
        type: 'ERROR',
        timestamp: expect.any(String),
        requestId: 'r-9',
        data: {
          code: 'RATE_LIMITED',
          message: 'slow down',
          retryable: true,
        },
      });
      expect(adapter.logger.error).not.toHaveBeenCalled();
    });

    it('swallows unexpected handler errors and logs them', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () => {
          throw new Error('boom');
        },
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));

      expect(client.send).not.toHaveBeenCalled();
      expect(adapter.logger.error).toHaveBeenCalledWith(
        expect.stringContaining('Unhandled WS handler error: Error: boom'),
        expect.stringContaining('Error: boom'),
      );
    });

    it('logs when the failing value has no stack', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () => {
          throw { message: 'raw object' };
        },
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));

      expect(client.send).not.toHaveBeenCalled();
      expect(adapter.logger.error).toHaveBeenCalledWith(
        'Unhandled WS handler error: [object Object]',
        undefined,
      );
    });

    it('swallows handler results that are not observable inputs', () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () => ({ plain: true }),
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));

      expect(handler.callback).toHaveBeenCalledTimes(1);
      expect(client.send).not.toHaveBeenCalled();
      expect(adapter.logger.error).toHaveBeenCalledTimes(1);
    });
  });

  describe('bindMessageHandlers stream errors', () => {
    it('answers rejected handler promises with an internal error envelope', async () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () => Promise.reject(new Error('downstream exploded')),
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));
      await flush();

      expect(client.send).toHaveBeenCalledTimes(1);
      expect(readEnvelope(client)).toEqual({
        type: 'ERROR',
        timestamp: expect.any(String),
        data: {
          code: 'INTERNAL_ERROR',
          message: 'Internal server error',
          retryable: true,
        },
      });
      expect(adapter.logger.error).toHaveBeenCalledWith(
        expect.stringContaining('Unhandled WS handler error'),
        expect.anything(),
      );
    });

    it('answers rejected promises carrying a WsException with its own code', async () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () =>
          Promise.reject(
            new WsException(WsErrorCode.SERVICE_UNAVAILABLE, 'dispatch down', true),
          ),
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));
      await flush();

      expect(readEnvelope(client)).toEqual({
        type: 'ERROR',
        timestamp: expect.any(String),
        data: {
          code: 'SERVICE_UNAVAILABLE',
          message: 'dispatch down',
          retryable: true,
        },
      });
      expect(adapter.logger.error).not.toHaveBeenCalled();
    });

    it('drops the error envelope when the socket is closed', async () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      const [handler] = makeHandlers({
        PING: () => Promise.reject(new Error('too late')),
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.readyState = 3;
      client.emit('message', json({ type: 'PING' }));
      await flush();

      expect(client.send).not.toHaveBeenCalled();
    });

    it('logs when the error envelope cannot be sent', async () => {
      const adapter = makeAdapter();
      const client = new FakeClient();
      client.send.mockImplementation(() => {
        throw new Error('write blocked');
      });
      const [handler] = makeHandlers({
        PING: () => Promise.reject(new Error('downstream exploded')),
      });

      adapter.bindMessageHandlers(client, [handler], {});
      client.emit('message', json({ type: 'PING' }));
      await flush();

      expect(adapter.logger.warn).toHaveBeenCalledWith(
        'Failed to send error envelope: write blocked',
      );
    });
  });
});
