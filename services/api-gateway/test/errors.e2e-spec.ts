/**
 * The gateway's `formatError` — every branch of the normalisation logic:
 *   1. subgraph errors carrying the "new" flat extensions
 *      (`{ success: false, statusCode, error, timeStamp }`)
 *   2. legacy subgraph errors (`extensions.response.body.errors[0]`)
 *   3. gateway-level errors (validation / depth limit) → 400 envelope
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

describe('Gateway error formatting (formatError)', () => {
  let h: GatewayHarness;

  beforeAll(async () => {
    h = await createGatewayHarness();
  });

  afterAll(async () => {
    await h.close();
  });

  afterEach(() => {
    h.subgraphs.clearErrors();
  });

  describe('branch 1: subgraph error with new-format extensions', () => {
    it('normalises to { success, statusCode, message, timeStamp, error }', async () => {
      h.subgraphs.injectError('delivery', 'delivery', {
        message: 'Delivery not found',
        extensions: {
          success: false,
          statusCode: 404,
          error: 'Not Found',
          timeStamp: '2026-01-01T00:00:00.000Z',
        },
      });

      const res = await h
        .gql(`{ delivery(id: "missing") { success } }`)
        .expect(200);

      // `delivery` is nullable → the failed field is nulled out, siblings kept
      expect(res.body.data).toEqual({ delivery: null });
      expect(res.body.errors).toHaveLength(1);
      expect(res.body.errors[0]).toEqual({
        success: false,
        statusCode: 404,
        message: 'Delivery not found',
        timeStamp: '2026-01-01T00:00:00.000Z',
        error: 'Not Found',
      });
    });

    it('defaults timeStamp when the subgraph omits it', async () => {
      h.subgraphs.injectError('driver', 'myDriverProfile', {
        message: 'Driver profile missing',
        extensions: { success: false, statusCode: 403, error: 'Forbidden' },
      });

      const res = await h.gql(`{ myDriverProfile { success } }`).expect(200);

      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 403,
        message: 'Driver profile missing',
        error: 'Forbidden',
      });
      expect(typeof res.body.errors[0].timeStamp).toBe('string');
      expect(Number.isNaN(Date.parse(res.body.errors[0].timeStamp))).toBe(
        false,
      );
    });

    it('does not mask the partial data of sibling fields', async () => {
      h.subgraphs.injectError('notification', 'myNotifications', {
        message: 'Notification store unavailable',
        extensions: { success: false, statusCode: 503 },
      });

      const res = await h
        .gql(`{ pingForUser { success } myNotifications { success } }`)
        .expect(200);

      // the healthy field still resolves alongside the error
      expect(res.body.data.pingForUser).toEqual({ success: true });
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 503,
        message: 'Notification store unavailable',
      });
    });
  });

  describe('branch 2: legacy subgraph error shape', () => {
    it('unwraps extensions.response.body.errors[0]', async () => {
      h.subgraphs.injectError('user', 'myProfile', {
        message: 'wrapped legacy failure',
        extensions: {
          response: {
            body: {
              errors: [
                {
                  message: 'User service is down',
                  statusCode: 502,
                  timeStamp: '2026-02-02T02:02:02.000Z',
                },
              ],
            },
          },
        },
      });

      const res = await h.gql(`{ myProfile { success } }`).expect(200);

      expect(res.body.errors[0]).toEqual({
        success: false,
        statusCode: 502,
        message: 'User service is down',
        timeStamp: '2026-02-02T02:02:02.000Z',
      });
    });

    it('defaults the legacy statusCode to 400 when absent', async () => {
      h.subgraphs.injectError('media', 'quota', {
        message: 'legacy without status',
        extensions: {
          response: {
            body: { errors: [{ message: 'legacy without status' }] },
          },
        },
      });

      const res = await h.gql(`{ quota { success } }`).expect(200);

      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
        message: 'legacy without status',
      });
      expect(typeof res.body.errors[0].timeStamp).toBe('string');
    });
  });

  describe('branch 3: gateway-level errors', () => {
    it('turns an unknown-field validation error into a 400 envelope', async () => {
      const res = await h.gql(`{ totallyUnknownField }`).expect(400);

      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
        message: expect.stringContaining('totallyUnknownField'),
      });
      expect(typeof res.body.errors[0].timeStamp).toBe('string');
    });

    it('turns a bad-argument validation error into a 400 envelope', async () => {
      // `id` is ID! — a Boolean literal cannot be coerced to ID
      const res = await h.gql(`{ user(id: true) { success } }`).expect(400);

      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
      expect(res.body.errors[0].message).toMatch(/ID|Boolean/i);
    });

    it('formats depth-limit violations', async () => {
      // Delivery → User.deliveries → Delivery … is cyclic; 8 levels > 7
      const deep = `{
        delivery(id: "d-1") {
          data {
            customer {
              deliveries {
                customer {
                  deliveries {
                    customer {
                      deliveries { id }
                    }
                  }
                }
              }
            }
          }
        }
      }`;

      const res = await h.gql(deep).expect(400);

      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
        message: expect.stringContaining(
          'exceeds maximum operation depth of 7',
        ),
      });
    });

    it('returns HTTP 400 with a formatted error for a syntax error', async () => {
      const res = await h.gql(`{ pingForUser {`).expect(400);

      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
      expect(res.body.errors[0].message).toMatch(/syntax|parse/i);
    });

    it('always includes message, statusCode, success and timeStamp keys', async () => {
      const res = await h.gql(`{ nope }`).expect(400);

      const err = res.body.errors[0];
      expect(Object.keys(err).sort()).toEqual(
        expect.arrayContaining([
          'message',
          'statusCode',
          'success',
          'timeStamp',
        ]),
      );
    });
  });
});
