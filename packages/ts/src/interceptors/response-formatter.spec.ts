import { ResponseFormatter } from './response-formatter';

describe('ResponseFormatter', () => {
  describe('formatSuccess', () => {
    it('wraps a plain object', () => {
      const result = ResponseFormatter.formatSuccess({ id: '1' });

      expect(result.success).toBe(true);
      expect(result.statusCode).toBe(200);
      expect(result.message).toBe('Request successful');
      expect(result.data).toEqual({ id: '1' });
      expect(result.items).toBeUndefined();
      expect(result.pagination).toBeUndefined();
      expect(new Date(result.timeStamp).toISOString()).toBe(result.timeStamp);
    });

    it('uses message from payload', () => {
      const result = ResponseFormatter.formatSuccess({ message: 'Saved', id: '1' });

      expect(result.message).toBe('Saved');
      expect(result.data).toEqual({ message: 'Saved', id: '1' });
    });

    it('unwraps nested data property', () => {
      const result = ResponseFormatter.formatSuccess({ data: { id: '9' } });

      expect(result.data).toEqual({ id: '9' });
    });

    it('keeps arrays as data and items', () => {
      const result = ResponseFormatter.formatSuccess([1, 2]);

      expect(result.data).toEqual([1, 2]);
      expect(result.items).toEqual([1, 2]);
      expect(result.message).toBe('Request successful');
    });

    it('extracts items from top level list', () => {
      const result = ResponseFormatter.formatSuccess({ items: ['a', 'b'], total: 2 });

      expect(result.items).toEqual(['a', 'b']);
      expect(result.data).toEqual({ items: ['a', 'b'], total: 2 });
      expect(result.pagination).toBeUndefined();
    });

    it('extracts items from nested data list', () => {
      const result = ResponseFormatter.formatSuccess({ data: { items: ['x'] } });

      expect(result.items).toEqual(['x']);
      expect(result.data).toEqual({ items: ['x'] });
    });

    it('omits items when the list is empty', () => {
      const result = ResponseFormatter.formatSuccess({ items: [] });

      expect(result.items).toBeUndefined();
    });

    it('preserves pagination metadata', () => {
      const result = ResponseFormatter.formatSuccess({
        items: ['a'],
        pagination: { page: 1, limit: 10 },
      });

      expect(result.pagination).toEqual({ page: 1, limit: 10 });
    });

    it('keeps envelope fields when payload is already formatted', () => {
      const result = ResponseFormatter.formatSuccess({
        success: true,
        statusCode: 201,
        message: 'Created',
        data: { id: '5' },
      });

      expect(result.statusCode).toBe(201);
      expect(result.message).toBe('Created');
      expect(result.data).toEqual({ id: '5' });
      expect(result.success).toBe(true);
    });

    it('keeps envelope message even when it is empty', () => {
      const result = ResponseFormatter.formatSuccess({
        success: true,
        statusCode: 204,
        data: null,
      });

      expect(result.statusCode).toBe(204);
      expect(result.message).toBeUndefined();
      expect(result.data).toBeNull();
    });

    it('handles null payload', () => {
      const result = ResponseFormatter.formatSuccess(null);

      expect(result.success).toBe(true);
      expect(result.statusCode).toBe(200);
      expect(result.message).toBe('Request successful');
      expect(result.data).toBeNull();
      expect(result.items).toBeUndefined();
    });
  });

  describe('formatError', () => {
    it('joins multiple validation messages', () => {
      const result = ResponseFormatter.formatError({
        errors: [{ message: 'name is required' }, { message: 'email is invalid' }],
      });

      expect(result.success).toBe(false);
      expect(result.message).toBe('name is required, email is invalid');
      expect(result.statusCode).toBe(500);
      expect(result.error).toBe('Unknown error');
      expect(new Date(result.timeStamp).toISOString()).toBe(result.timeStamp);
    });

    it('prefers response payload fields', () => {
      const result = ResponseFormatter.formatError({
        response: { message: 'Not Found', statusCode: 404, error: 'Not Found' },
        message: 'ignored',
      });

      expect(result.message).toBe('Not Found');
      expect(result.statusCode).toBe(404);
      expect(result.error).toBe('Not Found');
    });

    it('falls back to error message and status', () => {
      const result = ResponseFormatter.formatError({ message: 'boom', status: 403 });

      expect(result.message).toBe('boom');
      expect(result.statusCode).toBe(403);
      expect(result.error).toBe('Unknown error');
    });

    it('falls back to statusCode property', () => {
      const result = ResponseFormatter.formatError({ statusCode: 429 });

      expect(result.statusCode).toBe(429);
      expect(result.message).toBe('An unexpected error occurred');
    });

    it('uses first entry when message is an array', () => {
      const result = ResponseFormatter.formatError({ message: ['first', 'second'] });

      expect(result.message).toBe('first');
    });

    it('uses default values when nothing is provided', () => {
      const result = ResponseFormatter.formatError(undefined);

      expect(result.success).toBe(false);
      expect(result.statusCode).toBe(500);
      expect(result.message).toBe('An unexpected error occurred');
      expect(result.error).toBe('Unknown error');
      expect(result.timeStamp).toBeTruthy();
    });
  });
});
