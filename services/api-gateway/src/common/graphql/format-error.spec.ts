import { formatGatewayError } from './format-error';

describe('formatGatewayError', () => {
  describe('new-format subgraph errors (extensions.success === false + statusCode)', () => {
    it('maps a subgraph error into the envelope', () => {
      const error = {
        message: 'Not authorized',
        extensions: {
          success: false,
          statusCode: 403,
          timeStamp: '2026-01-01T00:00:00.000Z',
          error: 'Forbidden',
        },
      };

      expect(formatGatewayError(error)).toEqual({
        success: false,
        statusCode: 403,
        message: 'Not authorized',
        timeStamp: '2026-01-01T00:00:00.000Z',
        error: 'Forbidden',
      });
    });

    it('generates a timestamp when the subgraph omits one', () => {
      const error = {
        message: 'Bad',
        extensions: { success: false, statusCode: 400, error: 'Bad Request' },
      };

      const result = formatGatewayError(error);

      expect(result.timeStamp).toBeDefined();
      expect(new Date(result.timeStamp).toISOString()).toBe(result.timeStamp);
    });

    it('keeps a falsy error field as-is', () => {
      const error = {
        message: 'x',
        extensions: { success: false, statusCode: 422, error: undefined },
      };

      expect(formatGatewayError(error).error).toBeUndefined();
    });
  });

  describe('legacy subgraph error format', () => {
    it('maps extensions.response.body.errors[0]', () => {
      const error = {
        message: 'outer',
        extensions: {
          response: {
            body: {
              errors: [
                {
                  message: 'legacy message',
                  statusCode: 404,
                  timeStamp: '2026-02-02T00:00:00.000Z',
                },
              ],
            },
          },
        },
      };

      expect(formatGatewayError(error)).toEqual({
        success: false,
        statusCode: 404,
        message: 'legacy message',
        timeStamp: '2026-02-02T00:00:00.000Z',
      });
    });

    it('defaults statusCode to 400 when the legacy error omits it', () => {
      const error = {
        message: 'outer',
        extensions: {
          response: { body: { errors: [{ message: 'no code' }] } },
        },
      };

      const result = formatGatewayError(error);

      expect(result.statusCode).toBe(400);
      expect(result.message).toBe('no code');
      expect(result.timeStamp).toBeDefined();
    });
  });

  describe('gateway-level errors', () => {
    it('prefers extensions.statusCode and the message', () => {
      const error = {
        message: 'gateway boom',
        extensions: { statusCode: 502 },
      };

      const result = formatGatewayError(error);

      expect(result).toMatchObject({
        success: false,
        statusCode: 502,
        message: 'gateway boom',
      });
    });

    it('falls back to originalError.statusCode when extensions.statusCode is absent', () => {
      const error = {
        message: 'inner',
        extensions: {
          originalError: { message: 'inner msg', statusCode: 500 },
        },
      };

      const result = formatGatewayError(error);

      expect(result.statusCode).toBe(500);
      expect(result.message).toBe('inner msg');
    });

    it('defaults statusCode to 400 when neither is present', () => {
      const error = { message: 'plain' };

      const result = formatGatewayError(error);

      expect(result.statusCode).toBe(400);
      expect(result.message).toBe('plain');
    });

    it('takes the first element when the message is an array', () => {
      const error = {
        message: ['first error', 'second error'],
        extensions: {},
      };

      const result = formatGatewayError(error);

      expect(result.message).toBe('first error');
    });

    it('always returns success:false and a timestamp', () => {
      const result = formatGatewayError({ message: 'x' });

      expect(result.success).toBe(false);
      expect(new Date(result.timeStamp).toISOString()).toBe(result.timeStamp);
    });

    it('prefers originalError.message over error.message', () => {
      const error = {
        message: 'outer message',
        extensions: { originalError: { message: 'inner message' } },
      };

      expect(formatGatewayError(error).message).toBe('inner message');
    });
  });

  it('does not throw for an error with no extensions', () => {
    expect(() => formatGatewayError({ message: 'bare' })).not.toThrow();
  });
});
