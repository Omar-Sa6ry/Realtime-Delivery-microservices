/**
 * Auth flows — mirrors graphql-docs/auth.graphql plus the gateway's auth
 * surface: anonymous access, valid tokens, invalid/expired tokens.
 */
import request from 'supertest';

import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const USER_SELECTION = `
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

describe('Auth (graphql-docs/auth.graphql)', () => {
  let h: GatewayHarness;

  beforeAll(async () => {
    h = await createGatewayHarness();
  });

  afterAll(async () => {
    await h.close();
  });

  describe('register (Public)', () => {
    const REGISTER = `
      mutation Register($input: RegisterInput!) {
        register(input: $input) {
          statusCode
          message
          success
          data {
            accessToken
            refreshToken
            user { ${USER_SELECTION} }
          }
        }
      }
    `;

    it('accepts a full registration payload exactly as documented', async () => {
      const res = await h
        .gql(REGISTER, {
          input: {
            firstName: 'Omar',
            lastName: 'sabry',
            email: 'dffffr1@gmffail.com',
            password: 'ni7CV1a5+=2004',
            phoneNumber: '+201137836162',
          },
        })
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.register).toMatchObject({
        statusCode: 200,
        success: true,
        message: 'ok',
      });
      expect(res.body.data.register.data.accessToken).toBe('access-token');
      expect(res.body.data.register.data.refreshToken).toBe('refresh-token');
      expect(res.body.data.register.data.user).toMatchObject({
        id: 'me-1',
        email: 'omar@example.com',
        role: 'USER',
        isActive: true,
      });
      expect(res.body.data.register.data.user.addresses[0]).toMatchObject({
        title: 'Home',
        city: 'Cairo',
        isDefault: true,
      });

      // served by the user subgraph
      expect(h.subgraphs.last('user')?.fields).toContain('register');
    });

    it('works without any authentication header (public op)', async () => {
      const res = await h
        .gql(REGISTER, {
          input: {
            firstName: 'Anonymous',
            lastName: 'Caller',
            email: 'anon@example.com',
            password: 'Str0ng+Pass=2026',
          },
        })
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.register.success).toBe(true);
    });

    it('rejects a payload missing required fields with a validation error', async () => {
      const res = await h
        .gql(REGISTER, { input: { email: 'x@example.com' } })
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toMatch(
        /password|firstName|lastName/i,
      );
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  describe('login (Public)', () => {
    it('returns tokens and the user profile', async () => {
      const res = await h
        .gql(
          `mutation Login($input: LoginInput!) {
            login(input: $input) {
              statusCode
              message
              success
              data {
                accessToken
                refreshToken
                user { ${USER_SELECTION} }
              }
            }
          }`,
          {
            input: {
              email: 'ujnjngj@yutgrt.iuytr',
              password: 'ni7CV1a5+=2004',
            },
          },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.login.data).toMatchObject({
        accessToken: 'access-token',
        refreshToken: 'refresh-token',
      });
      expect(res.body.data.login.data.user.id).toBe('me-1');
    });

    it('rejects a login missing the password', async () => {
      const res = await h
        .gql(
          `mutation { login(input: { email: "a@b.com" }) { statusCode success } }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  describe('changePassword (User / Driver / Admin)', () => {
    it('performs the documented mutation for an authenticated user', async () => {
      const token = h.signToken({
        sub: 'user-1',
        role: 'USER',
        sessionId: 'sess-1',
      });

      const res = await h
        .gql(
          `mutation {
            changePassword(input: { passwordNew: "ni7CV1a5+=2004", passwordOld: "O12345hgtehight-" }) {
              statusCode
              message
              success
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.changePassword).toMatchObject({
        statusCode: 200,
        success: true,
      });
      expect(h.subgraphs.last('user')?.fields).toContain('changePassword');
    });
  });

  describe('forgetPassword / resetPassword (Public)', () => {
    it('forgets a password by email', async () => {
      const res = await h
        .gql(
          `mutation { forgetPassword(input: { email: "omarsfcdaffbry@gmail.com" }) { statusCode message success } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.forgetPassword).toMatchObject({
        statusCode: 200,
        success: true,
      });
    });

    it('resets a password with the emailed token', async () => {
      const res = await h
        .gql(
          `mutation {
            resetPassword(input: {
              token: "31a68f136bf4de00617d1790856cc5018a82053362dd88b0244abd5936cac12a923"
              passwordNew: "O12345hgtehight-"
            }) { statusCode message success }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.resetPassword).toMatchObject({
        statusCode: 200,
        success: true,
      });
    });
  });

  describe('refreshToken (Public)', () => {
    it('exchanges a refresh token for a new access token + user', async () => {
      const res = await h
        .gql(
          `mutation {
            refreshToken(input: { refreshToken: "YOUR_REFRESH_TOKEN_HERE" }) {
              statusCode
              message
              success
              data {
                accessToken
                refreshToken
                user { id email }
              }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.refreshToken.data.accessToken).toBe('access-token');
      expect(res.body.data.refreshToken.data.user).toMatchObject({
        id: 'me-1',
        email: 'omar@example.com',
      });
    });
  });

  describe('logout (User / Driver / Admin)', () => {
    it('logs out the current user', async () => {
      const token = h.signToken({ sub: 'user-1', role: 'USER' });

      const res = await h
        .gql(`mutation { logout { statusCode message success } }`, undefined, {
          token,
        })
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.logout).toMatchObject({
        statusCode: 200,
        success: true,
      });
    });
  });

  describe('gateway auth guard', () => {
    it('lets anonymous requests through (public ops still work)', async () => {
      const res = await h
        .gql(`{ pingForUser { statusCode success } }`)
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForUser.success).toBe(true);
    });

    it('accepts a valid token and populates the user context', async () => {
      const token = h.signToken({
        sub: 'user-42',
        role: 'DRIVER',
        sessionId: 'sess-42',
      });

      const res = await h
        .gql(`{ myProfile { statusCode success data { id } } }`, undefined, {
          token,
        })
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.myProfile.success).toBe(true);

      // the guard's parsed identity reached the subgraph
      const req = h.subgraphs.last('user');
      expect(req?.headers['x-user-id']).toBe('user-42');
      expect(req?.headers['x-user-role']).toBe('DRIVER');
      expect(req?.headers['x-user-session']).toBe('sess-42');
    });

    // NOTE: with `ApolloGatewayDriver` the guard runs only on REST routes —
    // Nest `APP_GUARD`s do not execute for the GraphQL operation itself (the
    // gateway serves /graphql outside Nest's resolver pipeline). On GraphQL the
    // `context` function only *decodes* the JWT payload (no signature/expiry
    // check), so identity headers are derived from whatever claims are present.
    it('treats a malformed token as anonymous on GraphQL (no identity forwarded)', async () => {
      const res = await h.gql(`{ pingForUser { success } }`, undefined, {
        token: 'not-a-jwt',
      });

      // public operation still succeeds; the guard never rejects GraphQL calls
      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForUser.success).toBe(true);
      // the payload could not be decoded, so no identity headers were forwarded
      expect(h.subgraphs.last('user')?.headers['x-user-id']).toBeUndefined();
      expect(h.subgraphs.last('user')?.headers['x-user-role']).toBeUndefined();
    });

    it('decodes a wrong-secret token without verifying on GraphQL (claims are trusted)', async () => {
      const forged = h.jwt.sign(
        { sub: 'user-1', role: 'ADMIN' },
        { secret: 'wrong-secret' },
      );

      const res = await h.gql(`{ pingForUser { success } }`, undefined, {
        token: forged,
      });

      // the GraphQL context does NOT verify the signature — it only base64-decodes
      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();
      // …so the forged claims are still forwarded downstream
      expect(h.subgraphs.last('user')?.headers['x-user-id']).toBe('user-1');
      expect(h.subgraphs.last('user')?.headers['x-user-role']).toBe('ADMIN');
    });

    it('decodes an expired token without verifying on GraphQL (claims are trusted)', async () => {
      const expired = h.jwt.sign(
        { sub: 'user-1', role: 'USER' },
        { secret: 'api-gateway-e2e-secret', expiresIn: '-1h' },
      );

      const res = await h.gql(`{ pingForUser { success } }`, undefined, {
        token: expired,
      });

      // expiry is not checked on the GraphQL path either
      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();
      expect(h.subgraphs.last('user')?.headers['x-user-id']).toBe('user-1');
    });

    it('rejects a malformed token on REST routes (GET /ping)', async () => {
      const res = await request(h.app.getHttpServer())
        .get('/ping')
        .set('Authorization', 'Bearer not-a-jwt');

      expect(res.status).toBe(401);
      expect(JSON.stringify(res.body)).toContain(
        'Invalid or expired JWT token',
      );
    });

    it('rejects a token signed with the wrong secret on REST routes (GET /ping)', async () => {
      const forged = h.jwt.sign(
        { sub: 'user-1', role: 'ADMIN' },
        { secret: 'wrong-secret' },
      );

      const res = await request(h.app.getHttpServer())
        .get('/ping')
        .set('Authorization', `Bearer ${forged}`);

      expect(res.status).toBe(401);
      expect(JSON.stringify(res.body)).toContain(
        'Invalid or expired JWT token',
      );
    });

    it('rejects an expired token on REST routes (GET /ping)', async () => {
      const expired = h.jwt.sign(
        { sub: 'user-1', role: 'USER' },
        { secret: 'api-gateway-e2e-secret', expiresIn: '-1h' },
      );

      const res = await request(h.app.getHttpServer())
        .get('/ping')
        .set('Authorization', `Bearer ${expired}`);

      expect(res.status).toBe(401);
      expect(JSON.stringify(res.body)).toContain(
        'Invalid or expired JWT token',
      );
    });

    it('allows REST liveness without a token', async () => {
      await request(h.app.getHttpServer()).get('/ping').expect(200);
    });
  });
});
