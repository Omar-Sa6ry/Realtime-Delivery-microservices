/**
 * Delivery service operations — mirrors graphql-docs/delivery.graphql,
 * including cross-subgraph entity resolution (Delivery → Driver / User).
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const DELIVERY_FIELDS = `
  id
  driver { id }
  customer { id }
  status
  paymentStatus
  checkoutUrl
  amount
  currency
  pickupAddress { line1 city countryCode line2 state postalCode latitude longitude }
  dropoffAddress { line1 city countryCode }
  statusHistory { status changedBy note createdAt }
  pickedUpAt
  completedAt
  cancelledAt
  createdAt
  updatedAt
`;

describe('Delivery service (graphql-docs/delivery.graphql)', () => {
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

  describe('deliveryServiceInfo (Public)', () => {
    it('returns service metadata', async () => {
      const res = await h
        .gql(
          `query GetDeliveryServiceInfo {
            deliveryServiceInfo { success statusCode message timeStamp data { name version status } }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deliveryServiceInfo).toMatchObject({
        success: true,
        statusCode: 200,
        message: 'ok',
        timeStamp: '2026-01-01T00:00:00.000Z',
      });
      expect(res.body.data.deliveryServiceInfo.data).toMatchObject({
        name: 'delivery',
        version: '1.0.0',
        status: 'UP',
      });
      expect(h.subgraphs.last('delivery')?.fields).toContain(
        'deliveryServiceInfo',
      );
    });
  });

  describe('createDelivery (User)', () => {
    it('creates a delivery with pickup/dropoff and an idempotency key', async () => {
      const res = await h
        .gql(
          `mutation CreateDelivery($input: CreateDeliveryInput!) {
            createDelivery(input: $input) {
              success statusCode message timeStamp
              data { ${DELIVERY_FIELDS} }
            }
          }`,
          {
            input: {
              amount: '25.50',
              currency: 'USD',
              pickupAddress: {
                line1: '1 Main Street',
                line2: 'Floor 3, Apt 12',
                city: 'Cairo',
                state: 'Cairo',
                postalCode: '11511',
                countryCode: 'EG',
                latitude: 30.0444,
                longitude: 31.2357,
              },
              dropoffAddress: {
                line1: '2 Nile Corniche',
                line2: 'Tower A',
                city: 'Giza',
                state: 'Giza',
                postalCode: '12511',
                countryCode: 'EG',
                latitude: 30.0131,
                longitude: 31.2089,
              },
              idempotencyKey: 'decreate-12345678c9c01c23ef45r67wc8',
            },
          },
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();

      const created = res.body.data.createDelivery;
      expect(created).toMatchObject({ success: true, statusCode: 201 });
      expect(created.data).toMatchObject({
        id: 'delivery-new',
        status: 'PENDING_PAYMENT',
        paymentStatus: 'PENDING',
        amount: '25.50',
        currency: 'USD',
        checkoutUrl: 'https://pay.example.com/checkout',
      });
      expect(created.data.pickupAddress).toMatchObject({
        line1: '1 Main Street',
        city: 'Cairo',
        countryCode: 'EG',
        latitude: 30.0444,
        longitude: 31.2357,
      });
      expect(created.data.dropoffAddress).toMatchObject({
        line1: '2 Nile Corniche',
        city: 'Giza',
      });

      // Delivery → Driver / User are federated entity references
      expect(created.data.driver).toEqual({ id: 'driver-1' });
      expect(created.data.customer).toEqual({ id: 'customer-1' });
    });

    it('requires amount, currency and both addresses', async () => {
      const res = await h
        .gql(
          `mutation { createDelivery(input: { amount: "1" }) { statusCode } }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });

    it('forwards the caller identity when creating a delivery', async () => {
      await h
        .gql(
          `mutation {
            createDelivery(input: {
              amount: "1.00" currency: "USD"
              pickupAddress: { line1: "a", city: "Cairo" }
              dropoffAddress: { line1: "b", city: "Giza" }
            }) { statusCode }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(h.subgraphs.last('delivery')?.headers['x-user-id']).toBe(
        'customer-1',
      );
    });
  });

  describe('deliveryNextStatuses (User / Driver / Admin)', () => {
    it('lists allowed transitions', async () => {
      const res = await h
        .gql(
          `query GetDeliveryNextStatuses {
            deliveryNextStatuses(id: "365064478569533440") {
              success statusCode message timeStamp data { statuses }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deliveryNextStatuses.data.statuses).toEqual([
        'PAYMENT_CONFIRMED',
        'CANCELLED',
      ]);
    });
  });

  describe('transitionDelivery (Admin)', () => {
    it('moves a delivery through the state machine', async () => {
      const res = await h
        .gql(
          `mutation TransitionDelivery($input: TransitionDeliveryInput!) {
            transitionDelivery(input: $input) {
              success statusCode message timeStamp
              data { ${DELIVERY_FIELDS} }
            }
          }`,
          {
            input: {
              deliveryId: '365061659779141632',
              status: 'DRIVER_ACCEPTED',
              note: 'Driver assigned to pickup package',
            },
          },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.transitionDelivery.data;
      expect(data).toMatchObject({
        id: '365061659779141632',
        status: 'DRIVER_ACCEPTED',
      });
      expect(data.statusHistory[0]).toMatchObject({
        status: 'PENDING_PAYMENT',
        changedBy: 'system',
        note: 'created',
      });
    });

    it('rejects an unknown status enum value', async () => {
      const res = await h
        .gql(
          `mutation {
            transitionDelivery(input: { deliveryId: "1", status: "NOT_A_STATUS" }) { statusCode }
          }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toMatch(/NOT_A_STATUS|DeliveryStatus/);
    });
  });

  describe('cancelDelivery (User / Admin)', () => {
    it('cancels a delivery with a reason', async () => {
      const res = await h
        .gql(
          `mutation CancelDelivery {
            cancelDelivery(deliveryId: "365065405678489600", reason: "Customer requested cancellation before pickup") {
              success statusCode message timeStamp data { id status cancelledAt updatedAt }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.cancelDelivery.data).toMatchObject({
        id: '365065405678489600',
        status: 'CANCELLED',
        cancelledAt: '2026-01-01T00:00:00.000Z',
      });
    });

    it('works without a reason (optional arg)', async () => {
      const res = await h
        .gql(
          `mutation { cancelDelivery(deliveryId: "1") { statusCode success } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.cancelDelivery.success).toBe(true);
    });
  });

  describe('myDeliveries (User)', () => {
    it('returns a paginated list', async () => {
      const res = await h
        .gql(
          `query GetMyDeliveries {
            myDeliveries(page: 1, pageSize: 20) {
              success statusCode message timeStamp
              data {
                paginationInfo { totalItems currentPage nextPage }
                items { ${DELIVERY_FIELDS} }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.myDeliveries.data;
      expect(data.paginationInfo).toEqual({
        totalItems: 1,
        currentPage: 1,
        nextPage: null,
      });
      expect(data.items).toHaveLength(1);
      expect(data.items[0]).toMatchObject({
        id: 'delivery-1',
        status: 'PENDING_PAYMENT',
        amount: '25.50',
        currency: 'USD',
      });
    });
  });

  describe('delivery(id) (User / Driver / Admin)', () => {
    it('returns the full delivery with addresses and history', async () => {
      const res = await h
        .gql(
          `query GetDelivery {
            delivery(id: "365065405678489600") {
              success statusCode message timeStamp
              data { ${DELIVERY_FIELDS} }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.delivery.data;
      expect(data.id).toBe('365065405678489600');
      expect(data.pickupAddress).toMatchObject({
        city: 'Cairo',
        countryCode: 'EG',
      });
      expect(data.dropoffAddress).toMatchObject({
        city: 'Giza',
        countryCode: 'EG',
      });
      expect(data.statusHistory).toHaveLength(1);
    });
  });

  describe('openDeliveries (Driver / Admin)', () => {
    it('lists open/unassigned deliveries', async () => {
      const res = await h
        .gql(
          `query GetOpenDeliveries {
            openDeliveries(page: 1, pageSize: 20) {
              success statusCode message
              data {
                items { id status paymentStatus amount currency
                  pickupAddress { city countryCode latitude longitude }
                  dropoffAddress { city countryCode }
                  createdAt }
                paginationInfo { totalItems currentPage nextPage }
              }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.openDeliveries.data;
      expect(data.items[0]).toMatchObject({
        id: 'delivery-open',
        pickupAddress: { city: 'Cairo', countryCode: 'EG' },
      });
      expect(data.paginationInfo.totalItems).toBe(1);
    });
  });

  it('routes every delivery operation to the delivery subgraph', async () => {
    h.subgraphs.clear();

    await h.gql(`{ deliveryServiceInfo { success } }`).expect(200);
    await h.gql(`{ deliveryNextStatuses(id: "1") { success } }`).expect(200);
    await h.gql(`{ myDeliveries { success } }`).expect(200);
    await h.gql(`{ delivery(id: "1") { success } }`).expect(200);
    await h.gql(`{ openDeliveries { success } }`).expect(200);
    await h
      .gql(
        `mutation { createDelivery(input: { amount: "1" currency: "USD"
          pickupAddress: { line1: "a", city: "C" }
          dropoffAddress: { line1: "b", city: "D" } }) { success } }`,
      )
      .expect(200);
    await h
      .gql(
        `mutation { transitionDelivery(input: { deliveryId: "1", status: PICKED_UP }) { success } }`,
      )
      .expect(200);
    await h
      .gql(`mutation { cancelDelivery(deliveryId: "1") { success } }`)
      .expect(200);

    const fields = new Set(
      h.subgraphs.forSubgraph('delivery').flatMap((r) => r.fields),
    );
    for (const f of [
      'deliveryServiceInfo',
      'deliveryNextStatuses',
      'myDeliveries',
      'delivery',
      'openDeliveries',
      'createDelivery',
      'transitionDelivery',
      'cancelDelivery',
    ]) {
      expect(fields).toContain(f);
    }
  });
});
