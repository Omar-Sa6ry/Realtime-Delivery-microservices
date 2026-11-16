import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import { GraphQLModule } from '@nestjs/graphql';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';
import { JwtModule, JwtService } from '@nestjs/jwt';
import { I18nService } from 'nestjs-i18n';
import request from 'supertest';
import { App } from 'supertest/types';

import { AppResolver } from '../src/app.resolver';
import { ConnectionService } from '../src/modules/gateway/connection/connection.service';

const JWT_SECRET = 'realtime-e2e-secret';

jest.setTimeout(30000);

describe('Realtime GraphQL (e2e)', () => {
  let app: INestApplication<App>;
  let jwt: JwtService;
  let i18n: { t: jest.Mock; translate: jest.Mock };
  let connectionService: {
    getUserConnectionStatus: jest.Mock;
    getActiveConnectionCounts: jest.Mock;
  };

  const sign = (payload: Record<string, unknown>) =>
    jwt.sign(payload, { secret: JWT_SECRET });

  const adminToken = () =>
    sign({ sub: 'admin-1', role: 'admin', email: 'admin@x.com' });
  const driverToken = () =>
    sign({ sub: 'driver-1', role: 'driver', email: 'driver@x.com' });

  const gql = (
    query: string,
    options: { token?: string; headers?: Record<string, string> } = {},
  ) => {
    let req = request(app.getHttpServer()).post('/realtime/graphql');
    if (options.token)
      req = req.set('Authorization', `Bearer ${options.token}`);
    for (const [key, value] of Object.entries(options.headers ?? {})) {
      req = req.set(key, value);
    }
    return req.send({ query });
  };

  beforeAll(async () => {
    process.env.JWT_SECRET = JWT_SECRET;

    i18n = {
      t: jest.fn(async (key: string) => key),
      translate: jest.fn(async (key: string) => key),
    };
    connectionService = {
      getUserConnectionStatus: jest.fn(),
      getActiveConnectionCounts: jest.fn(),
    };

    const moduleFixture: TestingModule = await Test.createTestingModule({
      imports: [
        GraphQLModule.forRoot<ApolloFederationDriverConfig>({
          driver: ApolloFederationDriver,
          path: '/realtime/graphql',
          autoSchemaFile: { federation: 2 },
          context: ({ req }: { req: unknown }) => ({
            req,
            language:
              (req as { headers?: Record<string, string> })?.headers?.[
                'accept-language'
              ] || 'en',
          }),
          playground: false,
          debug: false,
        }),
        JwtModule.register({
          secret: JWT_SECRET,
          signOptions: { expiresIn: '1h' },
        }),
      ],
      providers: [
        AppResolver,
        { provide: ConnectionService, useValue: connectionService },
        { provide: I18nService, useValue: i18n },
        {
          provide: 'USER_SERVICE',
          useValue: { findById: jest.fn() },
        },
      ],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();
    jwt = app.get(JwtService);
  });

  afterAll(async () => {
    await app.close();
  });

  beforeEach(() => {
    jest.clearAllMocks();
    connectionService.getUserConnectionStatus.mockResolvedValue({
      isConnected: true,
      lastSeen: '2026-01-01T00:00:00.000Z',
      connectionCount: 2,
    });
    connectionService.getActiveConnectionCounts.mockResolvedValue({
      total: 3,
      byRole: { customers: 1, drivers: 1, admins: 1 },
    });
  });

  describe('pingForRealtime (public)', () => {
    it('answers without any authentication', async () => {
      const res = await gql(`
        query {
          pingForRealtime {
            success
            statusCode
            message
            data
          }
        }
      `);

      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForRealtime).toMatchObject({
        success: true,
        statusCode: 200,
        message: 'messages.service.running',
        data: true,
      });
    });

    it('translates using the accept-language header', async () => {
      await gql(
        `
          query {
            pingForRealtime {
              data
            }
          }
        `,
        { headers: { 'accept-language': 'fr' } },
      );

      expect(i18n.translate).toHaveBeenCalledWith('messages.service.running', {
        lang: 'fr',
      });
    });
  });

  describe('getConnectionStatus (@Auth)', () => {
    const query = `
      query {
        getConnectionStatus(userId: "driver-1") {
          success
          statusCode
          message
          data {
            isConnected
            lastSeen
            connectionCount
          }
        }
      }
    `;

    it('lets an admin inspect any user', async () => {
      const res = await gql(query, { token: adminToken() });

      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.getConnectionStatus).toMatchObject({
        success: true,
        statusCode: 200,
        message: 'messages.service.statusRetrieved',
        data: {
          isConnected: true,
          lastSeen: '2026-01-01T00:00:00.000Z',
          connectionCount: 2,
        },
      });
      expect(connectionService.getUserConnectionStatus).toHaveBeenCalledWith(
        'driver-1',
      );
    });

    it('lets a driver inspect their own status', async () => {
      const res = await gql(query, { token: driverToken() });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.getConnectionStatus.data).toMatchObject({
        isConnected: true,
        connectionCount: 2,
      });
    });

    it('forbids a driver from inspecting another user', async () => {
      const res = await gql(
        `
          query {
            getConnectionStatus(userId: "someone-else") {
              success
            }
          }
        `,
        { token: driverToken() },
      );

      expect(res.body.errors?.[0]?.message).toBe('messages.ws.unauthorized');
      expect(connectionService.getUserConnectionStatus).not.toHaveBeenCalled();
    });

    it('rejects requests without a token', async () => {
      const res = await gql(query);

      expect(res.body.errors?.[0]?.message).toBe('user.NO_TOKEN');
      expect(connectionService.getUserConnectionStatus).not.toHaveBeenCalled();
    });

    it('rejects malformed tokens', async () => {
      const res = await gql(query, { token: 'not-a-real-jwt' });

      expect(res.body.errors?.[0]?.message).toBe('user.INVALID_TOKEN');
    });
  });

  describe('getActiveConnections (@Auth, admin only)', () => {
    const query = `
      query {
        getActiveConnections {
          success
          statusCode
          message
          data {
            total
            byRole {
              customers
              drivers
              admins
            }
          }
        }
      }
    `;

    it('returns the connection counts to an admin', async () => {
      const res = await gql(query, { token: adminToken() });

      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.getActiveConnections).toMatchObject({
        success: true,
        statusCode: 200,
        message: 'messages.service.countsRetrieved',
        data: {
          total: 3,
          byRole: { customers: 1, drivers: 1, admins: 1 },
        },
      });
      expect(connectionService.getActiveConnectionCounts).toHaveBeenCalled();
    });

    it('forbids non-admin users', async () => {
      const res = await gql(query, { token: driverToken() });

      expect(res.body.errors?.[0]?.message).toBe('messages.ws.unauthorized');
      expect(
        connectionService.getActiveConnectionCounts,
      ).not.toHaveBeenCalled();
    });
  });
});
