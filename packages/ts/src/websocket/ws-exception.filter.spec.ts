jest.mock('@nestjs/websockets', () => ({
  BaseWsExceptionFilter: class {
    catch(_exception: unknown, _host: unknown): void {
      return undefined;
    }
  },
}));

import { Logger } from '@nestjs/common';
import { BaseWsExceptionFilter } from '@nestjs/websockets';
import { WsExceptionFilter } from './ws-exception.filter';
import { WsCloseCode, WsErrorCode, WsException } from './ws-errors';
import { ServerMessageType } from './realtime-message';

interface FakeClient {
  send?: jest.Mock;
  close?: jest.Mock;
  emit?: jest.Mock;
}

const makeHost = (client: unknown, data?: unknown) =>
  ({
    switchToWs: () => ({
      getClient: () => client,
      getData: () => data,
      getPattern: () => 'message',
    }),
    getType: () => 'ws',
    getArgByIndex: () => [],
  }) as any;

describe('WsExceptionFilter', () => {
  let filter: WsExceptionFilter;
  let baseCatchSpy: jest.SpyInstance;
  let warnSpy: jest.SpyInstance;

  beforeEach(() => {
    filter = new WsExceptionFilter();
    baseCatchSpy = jest
      .spyOn(BaseWsExceptionFilter.prototype, 'catch')
      .mockImplementation(() => undefined);
    warnSpy = jest.spyOn(Logger.prototype, 'warn').mockImplementation();
  });

  afterEach(() => {
    baseCatchSpy.mockRestore();
    warnSpy.mockRestore();
  });

  const readEnvelope = (client: FakeClient) =>
    JSON.parse(client.send!.mock.calls[0][0] as string);

  it('is defined', () => {
    expect(filter).toBeInstanceOf(WsExceptionFilter);
    expect(filter).toBeInstanceOf(BaseWsExceptionFilter);
  });

  it('sends an error envelope carrying the requestId from the message', () => {
    const client: FakeClient = { send: jest.fn(), close: jest.fn() };
    const exception = new WsException(
      WsErrorCode.INVALID_DELIVERY_ID,
      'bad delivery',
    );

    filter.catch(
      exception,
      makeHost(client, { requestId: 'req-7', type: 'LOCATION_UPDATE' }),
    );

    const envelope = readEnvelope(client);
    expect(envelope.type).toBe(ServerMessageType.ERROR);
    expect(envelope.requestId).toBe('req-7');
    expect(envelope.data).toEqual({
      code: 'INVALID_DELIVERY_ID',
      message: 'bad delivery',
      retryable: false,
      requestId: 'req-7',
    });
    expect(Number.isNaN(Date.parse(envelope.timestamp))).toBe(false);
    expect(client.close).not.toHaveBeenCalled();
    expect(baseCatchSpy).not.toHaveBeenCalled();
  });

  it('omits the requestId when the incoming data has none', () => {
    const client: FakeClient = { send: jest.fn(), close: jest.fn() };

    filter.catch(
      new WsException(WsErrorCode.NOT_FOUND, 'gone'),
      makeHost(client, { type: 'PING' }),
    );

    const envelope = readEnvelope(client);
    expect(envelope).not.toHaveProperty('requestId');
    expect(envelope.data).toEqual({
      code: 'NOT_FOUND',
      message: 'gone',
      retryable: false,
    });
  });

  it('handles a host without any data', () => {
    const client: FakeClient = { send: jest.fn(), close: jest.fn() };

    filter.catch(
      new WsException(WsErrorCode.INTERNAL_ERROR, 'boom'),
      makeHost(client, undefined),
    );

    const envelope = readEnvelope(client);
    expect(envelope).not.toHaveProperty('requestId');
    expect(envelope.data.code).toBe('INTERNAL_ERROR');
    expect(client.close).not.toHaveBeenCalled();
  });

  describe('close code mapping', () => {
    const cases: Array<[WsErrorCode, WsCloseCode]> = [
      [WsErrorCode.UNAUTHENTICATED, WsCloseCode.UNAUTHORIZED],
      [WsErrorCode.FORBIDDEN, WsCloseCode.FORBIDDEN],
      [WsErrorCode.RATE_LIMITED, WsCloseCode.RATE_LIMITED],
      [WsErrorCode.TOO_LARGE, WsCloseCode.PAYLOAD_TOO_LARGE],
    ];

    it.each(cases)('closes with %s using %s', (code, closeCode) => {
      const client: FakeClient = { send: jest.fn(), close: jest.fn() };

      filter.catch(new WsException(code), makeHost(client));

      expect(client.close).toHaveBeenCalledWith(closeCode, code);
      expect(client.send).toHaveBeenCalledTimes(1);
    });

    it('keeps the connection open for non-closing codes', () => {
      const client: FakeClient = { send: jest.fn(), close: jest.fn() };

      filter.catch(
        new WsException(WsErrorCode.STALE_COMMAND, 'stale'),
        makeHost(client),
      );

      expect(client.send).toHaveBeenCalledTimes(1);
      expect(client.close).not.toHaveBeenCalled();
    });

    it('does not fail when the client exposes no close method', () => {
      const client: FakeClient = { send: jest.fn() };

      expect(() =>
        filter.catch(new WsException(WsErrorCode.UNAUTHENTICATED), makeHost(client)),
      ).not.toThrow();
      expect(client.send).toHaveBeenCalledTimes(1);
    });
  });

  describe('fallback to the base filter', () => {
    it('falls back when the client cannot receive messages', () => {
      const client: FakeClient = { emit: jest.fn() };
      const exception = new WsException(WsErrorCode.FORBIDDEN);

      filter.catch(exception, makeHost(client));

      expect(baseCatchSpy).toHaveBeenCalledWith(exception, expect.anything());
      expect(client.emit).not.toHaveBeenCalled();
    });

    it('falls back when there is no client at all', () => {
      const exception = new WsException(WsErrorCode.FORBIDDEN);

      filter.catch(exception, makeHost(null));

      expect(baseCatchSpy).toHaveBeenCalledWith(exception, expect.anything());
    });

    it('falls back and logs when sending the envelope throws', () => {
      const client: FakeClient = {
        send: jest.fn(() => {
          throw new Error('socket destroyed');
        }),
        close: jest.fn(),
        emit: jest.fn(),
      };
      const exception = new WsException(WsErrorCode.SERVICE_UNAVAILABLE);

      filter.catch(exception, makeHost(client));

      expect(warnSpy).toHaveBeenCalledWith(
        'Failed to send WsException envelope: socket destroyed',
      );
      expect(baseCatchSpy).toHaveBeenCalledWith(exception, expect.anything());
      expect(client.close).not.toHaveBeenCalled();
    });

    it('does not close the socket when sending already failed', () => {
      const client: FakeClient = {
        send: jest.fn(() => {
          throw new Error('nope');
        }),
        close: jest.fn(),
        emit: jest.fn(),
      };

      filter.catch(new WsException(WsErrorCode.RATE_LIMITED), makeHost(client));

      expect(client.close).not.toHaveBeenCalled();
      expect(baseCatchSpy).toHaveBeenCalledTimes(1);
    });
  });

  it('serializes a retryable payload', () => {
    const client: FakeClient = { send: jest.fn(), close: jest.fn() };

    filter.catch(
      new WsException(WsErrorCode.RATE_LIMITED, 'slow down', true),
      makeHost(client, { requestId: 'r-3' }),
    );

    expect(readEnvelope(client).data).toEqual({
      code: 'RATE_LIMITED',
      message: 'slow down',
      retryable: true,
      requestId: 'r-3',
    });
  });
});
