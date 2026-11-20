import { RemoteGraphQLDataSource } from '@apollo/gateway';
import {
  willSendRequest,
  buildSubgraphService,
  buildService,
} from './subgraph-service';
import { CORRELATION_ID_HEADER } from '../middlewares/correlation-id.middleware';

function fakeToken(payload: Record<string, unknown>): string {
  const header = Buffer.from(
    JSON.stringify({ alg: 'HS256', typ: 'JWT' }),
  ).toString('base64');
  const body = Buffer.from(JSON.stringify(payload)).toString('base64');
  return `${header}.${body}.signature`;
}

/** A request double that records header writes. */
function makeRequest() {
  const store = new Map<string, string>();
  return {
    store,
    request: {
      http: {
        headers: {
          set: (k: string, v: string) => store.set(k, v),
        },
      },
    } as any,
  };
}

describe('willSendRequest', () => {
  it('forwards x-user-id / x-user-role / x-user-session from context.req.user', () => {
    const { request, store } = makeRequest();
    willSendRequest({
      request,
      context: {
        req: {
          user: { userId: 'u1', role: 'ADMIN', sessionId: 's1' },
          headers: {},
        },
      },
    });

    expect(store.get('x-user-id')).toBe('u1');
    expect(store.get('x-user-role')).toBe('ADMIN');
    expect(store.get('x-user-session')).toBe('s1');
  });

  it('omits x-user-session when the user has no sessionId', () => {
    const { request, store } = makeRequest();
    willSendRequest({
      request,
      context: { req: { user: { userId: 'u1', role: 'USER' }, headers: {} } },
    });

    expect(store.has('x-user-session')).toBe(false);
  });

  it('falls back to empty strings when userId/role are missing', () => {
    const { request, store } = makeRequest();
    willSendRequest({
      request,
      context: { req: { user: {}, headers: {} } },
    });

    expect(store.get('x-user-id')).toBe('');
    expect(store.get('x-user-role')).toBe('');
  });

  it('decodes the Bearer token when context.req.user is absent', () => {
    const { request, store } = makeRequest();
    const token = fakeToken({
      sub: 'decoded',
      role: 'DRIVER',
      sessionId: 'ds',
    });
    willSendRequest({
      request,
      context: { req: { headers: { authorization: `Bearer ${token}` } } },
    });

    expect(store.get('x-user-id')).toBe('decoded');
    expect(store.get('x-user-role')).toBe('DRIVER');
    expect(store.get('x-user-session')).toBe('ds');
  });

  it('falls back to payload.userId when sub is absent during decode', () => {
    const { request, store } = makeRequest();
    const token = fakeToken({ userId: 'byUser' });
    willSendRequest({
      request,
      context: { req: { headers: { authorization: `Bearer ${token}` } } },
    });

    expect(store.get('x-user-id')).toBe('byUser');
  });

  it('falls back to payload.id when sub and userId are absent during decode', () => {
    const { request, store } = makeRequest();
    const token = fakeToken({ id: 'byId' });
    willSendRequest({
      request,
      context: { req: { headers: { authorization: `Bearer ${token}` } } },
    });

    expect(store.get('x-user-id')).toBe('byId');
  });

  it('defaults role to USER during decode when role is absent', () => {
    const { request, store } = makeRequest();
    const token = fakeToken({ sub: 'u' });
    willSendRequest({
      request,
      context: { req: { headers: { authorization: `Bearer ${token}` } } },
    });

    expect(store.get('x-user-role')).toBe('USER');
  });

  it('ignores a malformed token without throwing', () => {
    const { request, store } = makeRequest();

    expect(() =>
      willSendRequest({
        request,
        context: { req: { headers: { authorization: 'Bearer junk' } } },
      }),
    ).not.toThrow();
    expect(store.has('x-user-id')).toBe(false);
  });

  it('forwards the correlation id header', () => {
    const { request, store } = makeRequest();
    willSendRequest({
      request,
      context: { req: { headers: { [CORRELATION_ID_HEADER]: 'corr-1' } } },
    });

    expect(store.get(CORRELATION_ID_HEADER)).toBe('corr-1');
  });

  it('forwards the authorization header', () => {
    const { request, store } = makeRequest();
    willSendRequest({
      request,
      context: { req: { headers: { authorization: 'Bearer abc' } } },
    });

    expect(store.get('authorization')).toBe('Bearer abc');
  });

  it('propagates x-lang to both x-lang and accept-language', () => {
    const { request, store } = makeRequest();
    willSendRequest({
      request,
      context: { req: { headers: { 'x-lang': 'ar-EG' } } },
    });

    expect(store.get('x-lang')).toBe('ar-EG');
    expect(store.get('accept-language')).toBe('ar-EG');
  });

  it('falls back to accept-language when x-lang is absent', () => {
    const { request, store } = makeRequest();
    willSendRequest({
      request,
      context: { req: { headers: { 'accept-language': 'fr-FR' } } },
    });

    expect(store.get('x-lang')).toBe('fr-FR');
    expect(store.get('accept-language')).toBe('fr-FR');
  });

  it('prefers x-lang over accept-language', () => {
    const { request, store } = makeRequest();
    willSendRequest({
      request,
      context: {
        req: { headers: { 'x-lang': 'de-DE', 'accept-language': 'fr-FR' } },
      },
    });

    expect(store.get('x-lang')).toBe('de-DE');
    expect(store.get('accept-language')).toBe('de-DE');
  });

  it('sets no lang headers when neither is present', () => {
    const { request, store } = makeRequest();
    willSendRequest({ request, context: { req: { headers: {} } } });

    expect(store.has('x-lang')).toBe(false);
    expect(store.has('accept-language')).toBe(false);
  });

  it('handles an empty context (no req) without throwing', () => {
    const { request } = makeRequest();

    expect(() => willSendRequest({ request, context: {} })).not.toThrow();
  });
});

describe('buildSubgraphService', () => {
  it('returns a RemoteGraphQLDataSource bound to the subgraph url', () => {
    const ds = buildSubgraphService({ url: 'http://user-srv:4001/graphql' });

    expect(ds).toBeInstanceOf(RemoteGraphQLDataSource);
  });

  it('wires willSendRequest onto the data source', () => {
    const ds: any = buildSubgraphService({ url: 'http://x' });

    expect(typeof ds.willSendRequest).toBe('function');
  });

  it('handles an undefined url without throwing', () => {
    expect(() => buildSubgraphService({})).not.toThrow();
  });

  it('is exported as buildService (the gateway config alias)', () => {
    expect(buildService).toBe(buildSubgraphService);
  });
});
