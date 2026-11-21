/**
 * User service operations — mirrors graphql-docs/user.graphql.
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const USER_FIELDS = `
  id
  firstName
  lastName
  email
  isActive
  imageUrl
  addresses {
    title
    street
    city
    state
    postalCode
    latitude
    longitude
    isDefault
    createdAt
  }
  role
  phoneNumber
  createdAt
`;

describe('User service (graphql-docs/user.graphql)', () => {
  let h: GatewayHarness;
  let token: string;

  beforeAll(async () => {
    h = await createGatewayHarness();
    token = h.signToken({ sub: 'user-1', role: 'USER', sessionId: 'sess-1' });
  });

  afterAll(async () => {
    await h.close();
  });

  describe('pingForUser (Public)', () => {
    it('answers the documented query', async () => {
      const res = await h
        .gql(`{ pingForUser { statusCode message success data } }`)
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForUser).toMatchObject({
        statusCode: 200,
        message: 'ok',
        success: true,
        data: true,
      });
      expect(h.subgraphs.last('user')?.fields).toContain('pingForUser');
    });
  });

  describe('myProfile (User / Driver / Admin)', () => {
    it('returns the full profile with addresses', async () => {
      const res = await h
        .gql(
          `{ myProfile { statusCode message success data { ${USER_FIELDS} } } }`,
          undefined,
          {
            token,
          },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.myProfile.data).toMatchObject({
        id: 'me-1',
        firstName: 'Omar',
        lastName: 'Sabry',
        email: 'omar@example.com',
        role: 'USER',
        isActive: true,
        phoneNumber: '+201137836162',
      });
      expect(res.body.data.myProfile.data.addresses[0]).toMatchObject({
        title: 'Home',
        street: 'El-Galaa Street',
        city: 'Cairo',
        state: 'Cairo Governorate',
        postalCode: '11511',
        latitude: 30.0444,
        longitude: 31.2357,
        isDefault: true,
      });
    });

    it('forwards the caller identity to the user subgraph', async () => {
      await h
        .gql(`{ myProfile { success } }`, undefined, { token })
        .expect(200);

      const req = h.subgraphs.last('user');
      expect(req?.headers['x-user-id']).toBe('user-1');
      expect(req?.headers['x-user-role']).toBe('USER');
      expect(req?.headers['x-user-session']).toBe('sess-1');
      expect(req?.headers['authorization']).toBe(`Bearer ${token}`);
    });
  });

  describe('user(id) (Admin)', () => {
    it('fetches a user by id', async () => {
      const res = await h
        .gql(
          `{ user(id: "367980963323449344") { statusCode message success data { ${USER_FIELDS} } } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      // the mock echoes the requested id back
      expect(res.body.data.user.data.id).toBe('367980963323449344');
      expect(res.body.data.user.data.email).toBe('omar@example.com');
    });

    it('requires the id argument', async () => {
      const res = await h.gql(`{ user { success } }`).expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toContain('id');
    });
  });

  describe('updateProfile (User / Driver / Admin)', () => {
    it('updates first/last name', async () => {
      const res = await h
        .gql(
          `mutation { updateProfile(input: { firstName: "Ahmed", lastName: "Sabry" }) {
            statusCode message success data { ${USER_FIELDS} }
          } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.updateProfile.data.id).toBe('me-1');
      expect(h.subgraphs.last('user')?.fields).toContain('updateProfile');
    });
  });

  describe('admin user management', () => {
    it('promoteUserToAdmin returns a boolean payload', async () => {
      const promoted = await h
        .gql(
          `mutation { promoteUserToAdmin(id: "367981481311604736") { statusCode message success data } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(promoted.body.errors).toBeUndefined();
      expect(promoted.body.data.promoteUserToAdmin).toMatchObject({
        statusCode: 200,
        success: true,
        data: true,
      });
    });

    it('toggleUserActive flips the active flag', async () => {
      const res = await h
        .gql(
          `mutation { toggleUserActive(id: "367981824183373824", isActive: false) { statusCode message success } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.toggleUserActive).toMatchObject({
        statusCode: 200,
        success: true,
      });
      expect(h.subgraphs.last('user')?.fields).toContain('toggleUserActive');
    });

    it('requires both id and isActive', async () => {
      const res = await h
        .gql(
          `mutation { toggleUserActive(id: "1") { statusCode } }`,
          undefined,
          { token },
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toContain('isActive');
    });

    it('deleteUser removes a user', async () => {
      const res = await h
        .gql(
          `mutation { deleteUser(id: "367981824183373824") { statusCode message success } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deleteUser).toMatchObject({
        statusCode: 200,
        success: true,
      });
    });
  });

  describe('address book', () => {
    it('myAddresses returns the address list', async () => {
      const res = await h
        .gql(
          `{ myAddresses { statusCode message success data {
            id title street city state postalCode latitude longitude isDefault createdAt
          } } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.myAddresses.data[0]).toMatchObject({
        id: 'addr-1',
        title: 'Home',
        street: 'El-Galaa Street',
        city: 'Cairo',
        isDefault: true,
      });
    });

    it('addAddress stores a new address', async () => {
      const res = await h
        .gql(
          `mutation { addAddress(input: {
            title: "Home"
            street: "El-Galaa Street"
            city: "Cairo"
            state: "Cairo Governorate"
            postalCode: "11511"
            isDefault: true
          }) { statusCode message success data { id title street city state postalCode latitude longitude isDefault createdAt } } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.addAddress).toMatchObject({
        statusCode: 201,
        success: true,
      });
      expect(res.body.data.addAddress.data).toMatchObject({
        id: 'addr-new',
        title: 'Home',
        street: 'El-Galaa Street',
        city: 'Cairo',
      });
    });

    it('addAddress requires title/street/city/state', async () => {
      const res = await h
        .gql(
          `mutation { addAddress(input: { title: "Only title" }) { statusCode } }`,
          undefined,
          { token },
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });

    it('deleteAddress removes an address', async () => {
      const res = await h
        .gql(
          `mutation { deleteAddress(addressId: "367982146935066624") { statusCode message success } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deleteAddress).toMatchObject({
        statusCode: 200,
        success: true,
      });
    });

    it('setDefaultAddress marks the default address', async () => {
      const res = await h
        .gql(
          `mutation { setDefaultAddress(addressId: "367982285561008128") { statusCode message success } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.setDefaultAddress).toMatchObject({
        statusCode: 200,
        success: true,
      });
    });
  });
});
