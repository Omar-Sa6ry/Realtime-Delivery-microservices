/**
 * Driver service operations — mirrors graphql-docs/driver.graphql.
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const DRIVER_FIELDS = `
  id
  user { id email firstName lastName role }
  status
  vehicleType
  plateNumber
  createdAt
  capacityKg
  capabilities
  isActive
  isBlocked
  serviceArea
  updatedAt
`;

const ASSIGNMENT_FIELDS = `
  id
  delivery { id }
  driver { id }
  status
  attemptNumber
  offeredAt
  expiresAt
  acceptedAt
  rejectedAt
  completedAt
  createdAt
  updatedAt
`;

describe('Driver service (graphql-docs/driver.graphql)', () => {
  let h: GatewayHarness;
  let token: string;

  beforeAll(async () => {
    h = await createGatewayHarness();
    token = h.signToken({
      sub: 'driver-1',
      role: 'DRIVER',
      sessionId: 'sess-d1',
    });
  });

  afterAll(async () => {
    await h.close();
  });

  describe('driverServiceInfo (Public)', () => {
    it('returns service metadata', async () => {
      const res = await h
        .gql(
          `query GetDriverServiceInfo {
            driverServiceInfo { success statusCode message timeStamp data { name version status } }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.driverServiceInfo.data).toMatchObject({
        name: 'driver',
        version: '1.0.0',
        status: 'UP',
      });
      expect(h.subgraphs.last('driver')?.fields).toContain('driverServiceInfo');
    });
  });

  describe('registerDriver (User / Admin)', () => {
    it('registers a driver with vehicle details', async () => {
      const res = await h
        .gql(
          `mutation RegisterDriver($input: RegisterDriverInput!) {
            registerDriver(input: $input) { success statusCode message data { ${DRIVER_FIELDS} } }
          }`,
          {
            input: {
              userId: '364851396022898688',
              plateNumber: 'ABC-123',
              capacityKg: 50,
              vehicleType: 'MOTORCYCLE',
              capabilities: ['STANDARD'],
              serviceArea: 'Cairo',
            },
          },
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.registerDriver.data).toMatchObject({
        id: 'driver-1',
        status: 'ONLINE',
        vehicleType: 'MOTORCYCLE',
        plateNumber: 'ABC-123',
        capacityKg: 50,
        isActive: true,
        isBlocked: false,
        serviceArea: 'Cairo',
      });
      // Driver.user is resolved through the user subgraph entity reference
      expect(res.body.data.registerDriver.data.user).toMatchObject({
        id: 'driver-user-1',
        email: 'omar@example.com',
        role: 'USER',
      });
    });

    it('rejects an invalid vehicle type', async () => {
      const res = await h
        .gql(
          `mutation { registerDriver(input: { userId: "1", plateNumber: "X", vehicleType: SUBMARINE }) { statusCode } }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toMatch(/SUBMARINE|VehicleType/);
    });
  });

  describe('driver(id) & myDriverProfile', () => {
    it('fetches a driver by id', async () => {
      const res = await h
        .gql(
          `query GetDriver {
            driver(id: "1fe315a0-8066-435b-a000-4602effa2356") { success statusCode message data { ${DRIVER_FIELDS} } }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.driver.data.id).toBe(
        '1fe315a0-8066-435b-a000-4602effa2356',
      );
      expect(res.body.data.driver.data.plateNumber).toBe('ABC-123');
    });

    it('returns the caller driver profile', async () => {
      const res = await h
        .gql(
          `query GetMyDriverProfile { myDriverProfile { success statusCode message data { ${DRIVER_FIELDS} } } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.myDriverProfile.data.id).toBe('driver-1');
      expect(h.subgraphs.last('driver')?.headers['x-user-id']).toBe('driver-1');
      expect(h.subgraphs.last('driver')?.headers['x-user-role']).toBe('DRIVER');
    });
  });

  describe('updateDriverProfile (Driver / Admin)', () => {
    it('updates vehicle details', async () => {
      const res = await h
        .gql(
          `mutation UpdateDriverProfile($input: UpdateDriverProfileInput!) {
            updateDriverProfile(input: $input) { success statusCode message data { ${DRIVER_FIELDS} } }
          }`,
          {
            input: {
              vehicleType: 'MOTORCYCLE',
              plateNumber: 'XYZ-456',
              capacityKg: 20,
              serviceArea: 'Giza',
              driverId: '1fe315a0-8066-435b-a000-4602effa2356',
            },
          },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.updateDriverProfile.data.id).toBe('driver-1');
    });

    it('requires driverId', async () => {
      const res = await h
        .gql(
          `mutation { updateDriverProfile(input: { plateNumber: "X" }) { statusCode } }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  describe('driver lifecycle mutations', () => {
    const cases: Array<[string, string, string]> = [
      [
        'ActivateDriver',
        `activateDriver(driverId: "598a51a0-240e-4154-82c8-1b040f0c36a4")`,
        'activateDriver',
      ],
      [
        'GoOnline',
        `goOnline(idempotencyKey: "go-onfline-dddddri-123-123d4s567d890")`,
        'goOnline',
      ],
      [
        'GoOffline',
        `goOffline(idempotencyKey: "go-offline-driver-123-1234567890")`,
        'goOffline',
      ],
      [
        'BlockDriver',
        `blockDriver(driverId: "c2075b3a-631c-4edf-a810-7e1eda187a92", reason: "Suspicious activity detected")`,
        'blockDriver',
      ],
      [
        'UnblockDriver',
        `unblockDriver(driverId: "c2075b3a-631c-4edf-a810-7e1eda187a92", reason: "Verification completed")`,
        'unblockDriver',
      ],
      [
        'SuspendDriver',
        `suspendDriver(driverId: "a1814250-c214-4b90-a8c3-d74ddd431025", reason: "Policy violation")`,
        'suspendDriver',
      ],
      [
        'DeactivateDriver',
        `deactivateDriver(driverId: "598a51a0-240e-4154-82c8-1b040f0c36a4")`,
        'deactivateDriver',
      ],
    ];

    for (const [name, call, field] of cases) {
      it(`${name} returns the updated driver`, async () => {
        const res = await h
          .gql(
            `mutation ${name} { ${call} { success statusCode message data { ${DRIVER_FIELDS} } } }`,
          )
          .expect(200);

        expect(res.body.errors).toBeUndefined();
        expect(res.body.data[field]).toMatchObject({
          success: true,
          statusCode: 200,
        });
        expect(res.body.data[field].data.id).toBe('driver-1');
        expect(res.body.data[field].data.user.email).toBe('omar@example.com');
      });
    }
  });

  describe('driverStatus (Driver / Admin)', () => {
    it('reports operational status', async () => {
      const res = await h
        .gql(
          `query GetDriverStatus {
            driverStatus(driverId: "1fe315a0-8066-435b-a000-4602effa2356") {
              success statusCode message
              data { status hasActiveAssignment activeDelivery { id } lastSeenAt }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.driverStatus.data).toMatchObject({
        status: 'ONLINE',
        hasActiveAssignment: false,
        activeDelivery: null,
        lastSeenAt: '2026-01-01T00:00:00.000Z',
      });
    });
  });

  describe('nearbyDrivers (User / Admin)', () => {
    it('returns nearby drivers with distances', async () => {
      const res = await h
        .gql(
          `query GetNearbyDrivers {
            nearbyDrivers(input: { latitude: 30.0131, longitude: 51.2089, radiusKm: 50.0, vehicleType: MOTORCYCLE, limit: 10 }) {
              success statusCode message timeStamp
              data {
                paginationInfo { totalItems currentPage nextPage }
                items {
                  driver { id isActive rating isBlocked status vehicleType plateNumber capacityKg capabilities serviceArea }
                  distanceMeters status vehicleType latitude longitude rating isBlocked
                }
              }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const item = res.body.data.nearbyDrivers.data.items[0];
      expect(item).toMatchObject({
        distanceMeters: 1420.5,
        status: 'ONLINE',
        vehicleType: 'MOTORCYCLE',
        latitude: 30.0444,
        longitude: 31.2357,
        rating: 4.8,
        isBlocked: false,
      });
      expect(item.driver).toMatchObject({ id: 'driver-1', rating: 4.8 });
    });

    it('requires latitude and longitude', async () => {
      const res = await h
        .gql(`query { nearbyDrivers(input: { radiusKm: 10 }) { statusCode } }`)
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  describe('assignment queries', () => {
    it('driverActiveAssignment returns the active assignment', async () => {
      const res = await h
        .gql(
          `query GetDriverActiveAssignment {
            driverActiveAssignment(driverId: "ec91cbe4-d3b7-44d1-bb1e-1dbc78da988e") {
              success statusCode message timeStamp data { ${ASSIGNMENT_FIELDS} }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.driverActiveAssignment.data).toMatchObject({
        id: 'assignment-1',
        status: 'OFFERED',
        attemptNumber: 1,
      });
    });

    it('assignment(id) returns an assignment by id', async () => {
      const res = await h
        .gql(
          `query GetAssignment {
            assignment(id: "365060439215706112-313509ab-be87-4b3a-a52a-d5ea8e7be271") {
              success statusCode message timeStamp data { ${ASSIGNMENT_FIELDS} }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.assignment.data.id).toBe('assignment-1');
    });

    it('dispatchAttempts(deliveryId) lists attempts (documented variable usage)', async () => {
      const res = await h
        .gql(
          `query GetDispatchAttempts($deliveryId: ID!) {
            dispatchAttempts(deliveryId: $deliveryId) {
              success statusCode message timeStamp
              data {
                paginationInfo { totalItems currentPage nextPage }
                items { id delivery { id } driver { id } distanceMeters attemptNumber result reason createdAt }
              }
            }
          }`,
          { deliveryId: '364857843767054336' },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const item = res.body.data.dispatchAttempts.data.items[0];
      expect(item).toMatchObject({
        id: 'attempt-1',
        distanceMeters: 1420.5,
        attemptNumber: 1,
        result: 'OFFERED',
        reason: null,
      });
      // Delivery & Driver entities are resolved by their owning subgraphs
      expect(item.delivery).toEqual({ id: 'delivery-1' });
      expect(item.driver).toEqual({ id: 'driver-1' });
    });
  });

  describe('assignment mutations', () => {
    it('acceptAssignment accepts an offer', async () => {
      const res = await h
        .gql(
          `mutation AcceptAssignment {
            acceptAssignment(assignmentId: "356872997950197760-598a51a0-240e-4154-82c8-1b040f0c36a4", idempotencyKey: "accept-assignment-123-driver-d456") {
              success statusCode message timeStamp data { ${ASSIGNMENT_FIELDS} }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.acceptAssignment.data).toMatchObject({
        status: 'ACCEPTED',
        attemptNumber: 1,
      });
    });

    it('rejectAssignment rejects an offer with a reason', async () => {
      const res = await h
        .gql(
          `mutation RejectAssignment {
            rejectAssignment(assignmentId: "356841535104356352-c54b0bbf-07fb-40f6-a80c-b5510d8ccf43", reason: "Too far from pickup", idempotencyKey: "reject-assignment-123-drsiver-456") {
              success statusCode message timeStamp data { ${ASSIGNMENT_FIELDS} }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.rejectAssignment.data.status).toBe('ACCEPTED');
    });

    it('selfAcceptDelivery claims an open delivery', async () => {
      const res = await h
        .gql(
          `mutation SelfAcceptDelivery {
            selfAcceptDelivery(deliveryId: "365061659779141632", idempotencyKey: "self-accept-354294205775876096-driver1") {
              success statusCode message timeStamp
              data { id deliveryId driverId status offeredAt acceptedAt expiresAt }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.selfAcceptDelivery.data).toMatchObject({
        id: 'assignment-1',
        deliveryId: '365061659779141632',
        driverId: 'driver-1',
        status: 'ACCEPTED',
      });
    });
  });

  describe('availableDeliveries (Driver)', () => {
    it('lists open deliveries for drivers', async () => {
      const res = await h
        .gql(
          `query GetDriverAvailableDeliveries {
            availableDeliveries {
              success statusCode message timeStamp
              data {
                totalItems
                items { id customerId status pickupCity pickupCountry dropoffCity dropoffCountry amount currency createdAt }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.availableDeliveries.data;
      expect(data.totalItems).toBe(1);
      expect(data.items[0]).toMatchObject({
        id: 'delivery-1',
        customerId: 'customer-1',
        pickupCity: 'Cairo',
        dropoffCity: 'Giza',
        amount: '25.50',
      });
    });
  });

  describe('driver reviews', () => {
    it('rateDriver records a review (documented variable usage)', async () => {
      const res = await h
        .gql(
          `mutation RateDriver($input: RateDriverInput!) {
            rateDriver(input: $input) {
              success statusCode message timeStamp
              data { id driver { id } user { id } delivery { id } rating comment createdAt }
            }
          }`,
          {
            input: {
              driverId: '598a51a0-240e-4154-82c8-1b040f0c36a4',
              deliveryId: '356872997950197760',
              rating: 4.5,
              comment: 'Excellent service!',
            },
          },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.rateDriver.data;
      expect(data).toMatchObject({
        id: 'review-1',
        rating: 4.5,
        comment: 'Excellent service!',
      });
      // federated stubs back to the driver / user / delivery services
      expect(data.driver).toEqual({
        id: '598a51a0-240e-4154-82c8-1b040f0c36a4',
      });
      expect(data.user).toEqual({ id: 'customer-1' });
      expect(data.delivery).toEqual({ id: '356872997950197760' });
    });

    it('driverReviews lists reviews with an average', async () => {
      const res = await h
        .gql(
          `query GetDriverReviews {
            driverReviews(driverId: "e4173f29-2c26-4f2c-8baf-21f2fa058b09", page: 1, limit: 10) {
              success statusCode message timeStamp
              data {
                paginationInfo { totalItems currentPage nextPage }
                items { id user { id } delivery { id } rating comment createdAt }
                averageRating
                totalReviews
              }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.driverReviews.data;
      expect(data.items[0]).toMatchObject({
        id: 'review-1',
        rating: 5,
        comment: 'Excellent service!',
      });
      expect(data.averageRating).toBe(5);
      expect(data.totalReviews).toBe(1);
    });
  });

  it('routes every driver operation to the driver subgraph', async () => {
    h.subgraphs.clear();

    await h.gql(`{ driverServiceInfo { success } }`).expect(200);
    await h.gql(`{ driver(id: "1") { success } }`).expect(200);
    await h.gql(`{ myDriverProfile { success } }`).expect(200);
    await h.gql(`{ driverStatus(driverId: "1") { success } }`).expect(200);
    await h
      .gql(
        `{ nearbyDrivers(input: { latitude: 1.0, longitude: 1.0 }) { success } }`,
      )
      .expect(200);
    await h
      .gql(`{ driverActiveAssignment(driverId: "1") { success } }`)
      .expect(200);
    await h
      .gql(`{ dispatchAttempts(deliveryId: "1") { success } }`)
      .expect(200);
    await h.gql(`{ assignment(id: "1") { success } }`).expect(200);
    await h.gql(`{ availableDeliveries { success } }`).expect(200);
    await h.gql(`{ driverReviews(driverId: "1") { success } }`).expect(200);
    await h.gql(`mutation { goOnline { success } }`).expect(200);
    await h.gql(`mutation { goOffline { success } }`).expect(200);
    await h
      .gql(`mutation { selfAcceptDelivery(deliveryId: "1") { success } }`)
      .expect(200);
    await h
      .gql(
        `mutation { rateDriver(input: { driverId: "1", deliveryId: "1", rating: 5 }) { success } }`,
      )
      .expect(200);

    const fields = new Set(
      h.subgraphs.forSubgraph('driver').flatMap((r) => r.fields),
    );
    for (const f of [
      'driverServiceInfo',
      'driver',
      'myDriverProfile',
      'driverStatus',
      'nearbyDrivers',
      'driverActiveAssignment',
      'dispatchAttempts',
      'assignment',
      'availableDeliveries',
      'driverReviews',
      'goOnline',
      'goOffline',
      'selfAcceptDelivery',
      'rateDriver',
    ]) {
      expect(fields).toContain(f);
    }
  });
});
