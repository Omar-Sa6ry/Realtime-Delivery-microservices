import { WsCloseCode, WsErrorCode, WsException, WsErrorPayload } from './ws-errors';

describe('WsErrorCode', () => {
  it('exposes every error code', () => {
    expect(Object.values(WsErrorCode)).toEqual([
      'UNAUTHENTICATED',
      'FORBIDDEN',
      'INVALID_MESSAGE',
      'INVALID_DELIVERY_ID',
      'NOT_FOUND',
      'RATE_LIMITED',
      'TOO_LARGE',
      'STALE_COMMAND',
      'SERVICE_UNAVAILABLE',
      'INTERNAL_ERROR',
    ]);
  });
});

describe('WsCloseCode', () => {
  it('maps close codes to websocket application codes', () => {
    expect(WsCloseCode.UNAUTHORIZED).toBe(4401);
    expect(WsCloseCode.FORBIDDEN).toBe(4403);
    expect(WsCloseCode.RATE_LIMITED).toBe(4408);
    expect(WsCloseCode.PAYLOAD_TOO_LARGE).toBe(4413);
    expect(WsCloseCode.INTERNAL_ERROR).toBe(4500);
    expect(Object.values(WsCloseCode).filter((v) => typeof v === 'number')).toEqual([
      4401, 4403, 4408, 4413, 4500,
    ]);
  });
});

describe('WsException', () => {
  it('is an Error subclass named WsException', () => {
    const err = new WsException(WsErrorCode.NOT_FOUND);

    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(WsException);
    expect(err.name).toBe('WsException');
    expect(err.stack).toContain('WsException');
  });

  it('falls back to the code as message when none is given', () => {
    const err = new WsException(WsErrorCode.NOT_FOUND);

    expect(err.message).toBe('NOT_FOUND');
    expect(err.code).toBe(WsErrorCode.NOT_FOUND);
    expect(err.retryable).toBe(false);
  });

  it('keeps an explicit message and retryable flag', () => {
    const err = new WsException(
      WsErrorCode.SERVICE_UNAVAILABLE,
      'dispatch is down',
      true,
    );

    expect(err.message).toBe('dispatch is down');
    expect(err.code).toBe(WsErrorCode.SERVICE_UNAVAILABLE);
    expect(err.retryable).toBe(true);
  });

  it('treats an empty message as missing and uses the code', () => {
    const err = new WsException(WsErrorCode.INTERNAL_ERROR, '');

    expect(err.message).toBe('INTERNAL_ERROR');
  });

  describe('toPayload', () => {
    it('serializes the full payload with a requestId', () => {
      const err = new WsException(WsErrorCode.RATE_LIMITED, 'slow down', true);

      const payload: WsErrorPayload = err.toPayload('req-9');

      expect(payload).toEqual({
        code: 'RATE_LIMITED',
        message: 'slow down',
        retryable: true,
        requestId: 'req-9',
      });
    });

    it('leaves requestId undefined when none is supplied', () => {
      const err = new WsException(WsErrorCode.INVALID_MESSAGE, 'bad json');

      expect(err.toPayload()).toEqual({
        code: 'INVALID_MESSAGE',
        message: 'bad json',
        retryable: false,
        requestId: undefined,
      });
      expect(JSON.parse(JSON.stringify(err.toPayload()))).toEqual({
        code: 'INVALID_MESSAGE',
        message: 'bad json',
        retryable: false,
      });
    });

    it('produces a JSON-serializable payload', () => {
      const err = new WsException(WsErrorCode.FORBIDDEN);

      expect(JSON.parse(JSON.stringify(err.toPayload('r-1')))).toEqual({
        code: 'FORBIDDEN',
        message: 'FORBIDDEN',
        retryable: false,
        requestId: 'r-1',
      });
    });
  });
});
