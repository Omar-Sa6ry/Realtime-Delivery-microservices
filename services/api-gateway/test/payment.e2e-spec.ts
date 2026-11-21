/**
 * Payment service operations — mirrors graphql-docs/payment.graphql, including
 * the federated Payment → Delivery / User entity stubs.
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const PAYMENT_FIELDS = `
  id
  deliveryId
  delivery { id }
  userId
  user { id }
  amountMinor
  currency
  status
  provider
  correlationId
  causationId
  createdAt
  updatedAt
  authorizedAt
  capturedAt
`;

describe('Payment service (graphql-docs/payment.graphql)', () => {
  let h: GatewayHarness;
  let token: string;

  beforeAll(async () => {
    h = await createGatewayHarness();
    token = h.signToken({
      sub: 'customer-1',
      role: 'USER',
      sessionId: 'sess-1',
    });
  });

  afterAll(async () => {
    await h.close();
  });

  describe('paymentServiceInfo (Public)', () => {
    it('returns service metadata', async () => {
      const res = await h
        .gql(
          `query GetPaymentServiceInfo {
            paymentServiceInfo { success statusCode message timeStamp data { name version status } }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.paymentServiceInfo.data).toMatchObject({
        name: 'payment',
        version: '1.0.0',
        status: 'UP',
      });
      expect(h.subgraphs.last('payment')?.fields).toContain(
        'paymentServiceInfo',
      );
    });
  });

  describe('payments (Admin)', () => {
    it('lists payments paginated with federated delivery/user stubs', async () => {
      const res = await h
        .gql(
          `query GetPaymentsList {
            payments(page: 1, limit: 10) {
              success statusCode message timeStamp
              data {
                paginationInfo { totalItems currentPage nextPage }
                items { ${PAYMENT_FIELDS} }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();

      const data = res.body.data.payments.data;
      expect(data.paginationInfo).toEqual({
        totalItems: 1,
        currentPage: 1,
        nextPage: null,
      });

      const payment = data.items[0];
      expect(payment).toMatchObject({
        id: 'payment-1',
        deliveryId: 'delivery-1',
        userId: 'customer-1',
        amountMinor: 2550,
        currency: 'USD',
        status: 'CAPTURED',
        provider: 'STRIPE',
        correlationId: 'corr-1',
        causationId: 'cause-1',
        authorizedAt: '2026-01-01T00:00:00.000Z',
        capturedAt: '2026-01-01T00:00:00.000Z',
      });
      // entity references resolved back through their owning subgraphs
      expect(payment.delivery).toEqual({ id: 'delivery-1' });
      expect(payment.user).toEqual({ id: 'customer-1' });
    });
  });

  describe('payment(id) (User / Driver / Admin)', () => {
    it('fetches a payment by id with federated stubs', async () => {
      const res = await h
        .gql(
          `query GetPaymentById {
            payment(id: "63359164469") {
              success statusCode message timeStamp
              data { ${PAYMENT_FIELDS} }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const payment = res.body.data.payment.data;
      expect(payment.id).toBe('63359164469');
      expect(payment.status).toBe('CAPTURED');
      expect(payment.delivery).toEqual({ id: 'delivery-1' });
      expect(payment.user).toEqual({ id: 'customer-1' });
    });

    it('forwards the caller identity to the payment subgraph', async () => {
      await h
        .gql(`{ payment(id: "1") { success } }`, undefined, { token })
        .expect(200);

      const req = h.subgraphs.last('payment');
      expect(req?.headers['x-user-id']).toBe('customer-1');
      expect(req?.headers['x-user-role']).toBe('USER');
      expect(req?.headers['authorization']).toBe(`Bearer ${token}`);
    });
  });
});
