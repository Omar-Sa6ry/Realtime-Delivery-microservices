import {
  CorrelationIdMiddleware,
  CORRELATION_ID_HEADER,
} from './correlation-id.middleware';

describe('CorrelationIdMiddleware', () => {
  let middleware: CorrelationIdMiddleware;
  let req: any;
  let res: any;
  let next: jest.Mock;

  beforeEach(() => {
    middleware = new CorrelationIdMiddleware();
    next = jest.fn();
    res = { setHeader: jest.fn() };
  });

  it('exports the correlation header name', () => {
    expect(CORRELATION_ID_HEADER).toBe('x-correlation-id');
  });

  describe('when no correlation id is present', () => {
    beforeEach(() => {
      req = { headers: {} };
    });

    it('generates a UUID v4 and stores it on req.headers', () => {
      middleware.use(req, res, next);

      const generated = req.headers[CORRELATION_ID_HEADER];
      expect(generated).toBeDefined();
      // RFC 4122 v4 shape
      expect(generated).toMatch(
        /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
      );
    });

    it('sets the response header to the generated id', () => {
      middleware.use(req, res, next);

      expect(res.setHeader).toHaveBeenCalledWith(
        CORRELATION_ID_HEADER,
        req.headers[CORRELATION_ID_HEADER],
      );
    });

    it('generates a different id on each call', () => {
      middleware.use(req, res, next);
      const first = req.headers[CORRELATION_ID_HEADER];

      const otherReq: any = { headers: {} };
      middleware.use(otherReq, res, next);

      expect(otherReq.headers[CORRELATION_ID_HEADER]).not.toBe(first);
    });

    it('calls next()', () => {
      middleware.use(req, res, next);
      expect(next).toHaveBeenCalledTimes(1);
    });
  });

  describe('when a correlation id is already present', () => {
    it('preserves the incoming id', () => {
      req = { headers: { [CORRELATION_ID_HEADER]: 'client-supplied-id' } };

      middleware.use(req, res, next);

      expect(req.headers[CORRELATION_ID_HEADER]).toBe('client-supplied-id');
    });

    it('echoes the incoming id on the response', () => {
      req = { headers: { [CORRELATION_ID_HEADER]: 'client-supplied-id' } };

      middleware.use(req, res, next);

      expect(res.setHeader).toHaveBeenCalledWith(
        CORRELATION_ID_HEADER,
        'client-supplied-id',
      );
    });

    it('still calls next()', () => {
      req = { headers: { [CORRELATION_ID_HEADER]: 'client-supplied-id' } };

      middleware.use(req, res, next);

      expect(next).toHaveBeenCalledTimes(1);
    });
  });
});
