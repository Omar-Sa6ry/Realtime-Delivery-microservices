/**
 * Search service operations — mirrors graphql-docs/search.graphql.
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const PAGINATION = `paginationInfo { totalItems currentPage nextPage }`;

describe('Search service (graphql-docs/search.graphql)', () => {
  let h: GatewayHarness;
  let token: string;

  beforeAll(async () => {
    h = await createGatewayHarness();
    token = h.signToken({ sub: 'user-1', role: 'ADMIN', sessionId: 'sess-1' });
  });

  afterAll(async () => {
    await h.close();
  });

  describe('searchHealth (Public)', () => {
    it('reports search health', async () => {
      const res = await h
        .gql(
          `query SearchHealth {
            searchHealth { success statusCode message timeStamp data { status timestamp } }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.searchHealth.data).toMatchObject({ status: 'UP' });
      expect(h.subgraphs.last('search')?.fields).toContain('searchHealth');
    });
  });

  describe('autocomplete (Public)', () => {
    it('returns suggestions for a prefix', async () => {
      const res = await h
        .gql(
          `query Autocomplete {
            autocomplete(input: { prefix: "Screen", index: "media", limit: 5 }) {
              success statusCode message timeStamp data { suggestions }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.autocomplete.data.suggestions).toEqual([
        'Screen',
        'Screenshot',
      ]);
    });

    it('requires the prefix', async () => {
      const res = await h
        .gql(`query { autocomplete(input: { index: "media" }) { statusCode } }`)
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  describe('searchMedia (User / Driver / Admin)', () => {
    it('searches media with pagination', async () => {
      const res = await h
        .gql(
          `query SearchMedia {
            searchMedia(input: { query: "Screen", pagination: { limit: 10 } }) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items { media_id file_name mime_type owner_id size status created_at }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.searchMedia.data;
      expect(data.paginationInfo.totalItems).toBe(1);
      expect(data.items[0]).toMatchObject({
        media_id: 'media-1',
        file_name: 'screen.png',
        mime_type: 'image/png',
        status: 'READY',
      });
    });
  });

  describe('searchUsers (Admin)', () => {
    it('searches users with role and active filters', async () => {
      const res = await h
        .gql(
          `query SearchUsers {
            searchUsers(input: { query: "omar", role: "user", isActive: true, pagination: { limit: 4 } }) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items { id first_name last_name email is_active created_at }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.searchUsers.data.items[0]).toMatchObject({
        id: 'u-1',
        first_name: 'Omar',
        last_name: 'Sabry',
        email: 'omar@example.com',
        is_active: true,
      });
    });
  });

  describe('searchDeliveries (Admin)', () => {
    it('searches deliveries with geo pickup/dropoff', async () => {
      const res = await h
        .gql(
          `query SearchDeliveries {
            searchDeliveries(input: { query: "Cairo", status: "CREATED", pagination: { limit: 10 } }) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items {
                  delivery_id customer_id driver_id status
                  pickup { city country location { lat lon } }
                  dropoff { city country location { lat lon } }
                  created_at updated_at source_version
                }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const item = res.body.data.searchDeliveries.data.items[0];
      expect(item).toMatchObject({
        delivery_id: 'delivery-1',
        customer_id: 'customer-1',
        driver_id: 'driver-1',
        status: 'PENDING_PAYMENT',
        pickup: {
          city: 'Cairo',
          country: 'EG',
          location: { lat: 30.0444, lon: 31.2357 },
        },
        dropoff: { city: 'Giza', country: 'EG' },
      });
    });
  });

  describe('nearbyDeliveries (Driver / Admin)', () => {
    it('finds deliveries near a coordinate', async () => {
      const res = await h
        .gql(
          `query NearbyDeliveries {
            nearbyDeliveries(input: { lat: 1.0, lon: 1.0, radiusKm: 1000.0, pagination: { limit: 10 } }) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items { delivery_id customer_id status pickup { city country location { lat lon } } }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.nearbyDeliveries.data.items[0]).toMatchObject({
        delivery_id: 'delivery-1',
        status: 'PENDING_PAYMENT',
        pickup: { city: 'Cairo', country: 'EG' },
      });
    });

    it('requires lat and lon', async () => {
      const res = await h
        .gql(
          `query { nearbyDeliveries(input: { radiusKm: 10 }) { statusCode } }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  describe('searchDrivers (Admin)', () => {
    it('searches drivers with geo and rating filters', async () => {
      const res = await h
        .gql(
          `query SearchDrivers {
            searchDrivers(input: {
              query: "omar"
              geo: { lat: 1.0, lon: 10.0, radiusKm: 10.0 }
              vehicleType: "CAR"
              status: "AVAILABLE"
              minRating: 1
              pagination: { limit: 10 }
            }) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items { driver_id name status vehicle_type rating location { lat lon } updated_at source_version }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.searchDrivers.data.items[0]).toMatchObject({
        driver_id: 'driver-1',
        name: 'Omar',
        status: 'ONLINE',
        vehicle_type: 'CAR',
        rating: 4.5,
        location: { lat: 1.0, lon: 10.0 },
      });
    });
  });

  describe('searchNearbyDrivers (User / Admin)', () => {
    it('finds drivers near a coordinate', async () => {
      const res = await h
        .gql(
          `query SearchNearbyDrivers {
            searchNearbyDrivers(input: { lat: 12.0, lon: 10.0, radiusKm: 10.0, pagination: { limit: 10 } }) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items { driver_id name status vehicle_type rating location { lat lon } }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.searchNearbyDrivers.data.items[0]).toMatchObject({
        driver_id: 'driver-1',
        vehicle_type: 'CAR',
        rating: 4.5,
      });
    });
  });

  describe('startReindex (Admin)', () => {
    it('kicks off a reindex job', async () => {
      const res = await h
        .gql(
          `mutation StartReindex {
            startReindex(index: "deliveries") {
              success statusCode message timeStamp
              data { jobId index status startedAt completedAt error }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.startReindex).toMatchObject({
        statusCode: 202,
        success: true,
      });
      expect(res.body.data.startReindex.data).toMatchObject({
        jobId: 'job-1',
        index: 'deliveries',
        status: 'STARTED',
        error: null,
      });
    });

    it('requires the index name', async () => {
      const res = await h
        .gql(`mutation { startReindex { statusCode } }`)
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  it('routes every search operation to the search subgraph', async () => {
    h.subgraphs.clear();

    await h.gql(`{ searchHealth { success } }`).expect(200);
    await h
      .gql(`{ autocomplete(input: { prefix: "a" }) { success } }`)
      .expect(200);
    await h
      .gql(`{ searchMedia(input: { query: "a" }) { success } }`)
      .expect(200);
    await h
      .gql(`{ searchUsers(input: { query: "a" }) { success } }`)
      .expect(200);
    await h
      .gql(`{ searchDeliveries(input: { query: "a" }) { success } }`)
      .expect(200);
    await h
      .gql(`{ nearbyDeliveries(input: { lat: 1.0, lon: 1.0 }) { success } }`)
      .expect(200);
    await h
      .gql(`{ searchDrivers(input: { query: "a" }) { success } }`)
      .expect(200);
    await h
      .gql(`{ searchNearbyDrivers(input: { lat: 1.0, lon: 1.0 }) { success } }`)
      .expect(200);
    await h
      .gql(`mutation { startReindex(index: "deliveries") { success } }`)
      .expect(200);

    const fields = new Set(
      h.subgraphs.forSubgraph('search').flatMap((r) => r.fields),
    );
    for (const f of [
      'searchHealth',
      'autocomplete',
      'searchMedia',
      'searchUsers',
      'searchDeliveries',
      'nearbyDeliveries',
      'searchDrivers',
      'searchNearbyDrivers',
      'startReindex',
    ]) {
      expect(fields).toContain(f);
    }
  });
});
