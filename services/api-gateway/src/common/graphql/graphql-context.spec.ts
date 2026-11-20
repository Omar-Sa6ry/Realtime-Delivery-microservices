import { applyUserFromToken, buildGraphQLContext } from './graphql-context';

/** Builds a fake "unverified" JWT: header.payload.signature (payload base64). */
function fakeToken(payload: Record<string, unknown>): string {
  const header = Buffer.from(
    JSON.stringify({ alg: 'HS256', typ: 'JWT' }),
  ).toString('base64');
  const body = Buffer.from(JSON.stringify(payload)).toString('base64');
  return `${header}.${body}.signature`;
}

describe('applyUserFromToken', () => {
  it('is a no-op when req is undefined', () => {
    expect(() => applyUserFromToken(undefined)).not.toThrow();
  });

  it('is a no-op when req.user is already set', () => {
    const req = {
      user: { userId: 'existing' },
      headers: { authorization: `Bearer ${fakeToken({ sub: 'new' })}` },
    };

    applyUserFromToken(req);

    expect(req.user).toEqual({ userId: 'existing' });
  });

  it('is a no-op without an Authorization header', () => {
    const req: any = { headers: {} };

    applyUserFromToken(req);

    expect(req.user).toBeUndefined();
  });

  it('is a no-op for a non-Bearer Authorization header', () => {
    const req: any = { headers: { authorization: 'Basic abc' } };

    applyUserFromToken(req);

    expect(req.user).toBeUndefined();
  });

  it('decodes a valid 3-part token and attaches the user', () => {
    const req: any = {
      headers: {
        authorization: `Bearer ${fakeToken({ sub: 'u1', role: 'ADMIN', sessionId: 's1' })}`,
      },
    };

    applyUserFromToken(req);

    expect(req.user).toEqual({ userId: 'u1', role: 'ADMIN', sessionId: 's1' });
  });

  it('maps userId from payload.userId when sub is absent', () => {
    const req: any = {
      headers: { authorization: `Bearer ${fakeToken({ userId: 'byUser' })}` },
    };

    applyUserFromToken(req);

    expect(req.user.userId).toBe('byUser');
  });

  it('maps userId from payload.id when sub and userId are absent', () => {
    const req: any = {
      headers: { authorization: `Bearer ${fakeToken({ id: 'byId' })}` },
    };

    applyUserFromToken(req);

    expect(req.user.userId).toBe('byId');
  });

  it('defaults role to USER when role is absent', () => {
    const req: any = {
      headers: { authorization: `Bearer ${fakeToken({ sub: 'u' })}` },
    };

    applyUserFromToken(req);

    expect(req.user.role).toBe('USER');
  });

  it('leaves sessionId undefined when absent', () => {
    const req: any = {
      headers: { authorization: `Bearer ${fakeToken({ sub: 'u' })}` },
    };

    applyUserFromToken(req);

    expect(req.user.sessionId).toBeUndefined();
  });

  it('ignores a token that does not have 3 parts', () => {
    const req: any = { headers: { authorization: 'Bearer onlyonepart' } };

    applyUserFromToken(req);

    expect(req.user).toBeUndefined();
  });

  it('ignores a token whose payload is not valid JSON', () => {
    const header = Buffer.from('{}').toString('base64');
    const badPayload = Buffer.from('not-json').toString('base64');
    const req: any = {
      headers: { authorization: `Bearer ${header}.${badPayload}.sig` },
    };

    applyUserFromToken(req);

    expect(req.user).toBeUndefined();
  });
});

describe('buildGraphQLContext', () => {
  it('returns { req } for an anonymous request', () => {
    const req: any = { headers: {} };

    const ctx = buildGraphQLContext({ req });

    expect(ctx).toEqual({ req });
  });

  it('attaches the decoded user to req and returns it in context', () => {
    const req: any = {
      headers: {
        authorization: `Bearer ${fakeToken({ sub: 'ctx-user', role: 'DRIVER' })}`,
      },
    };

    const ctx = buildGraphQLContext({ req });

    expect(ctx.req.user).toEqual({ userId: 'ctx-user', role: 'DRIVER' });
  });

  it('handles a missing req gracefully', () => {
    const ctx = buildGraphQLContext({} as any);

    expect(ctx.req).toBeUndefined();
  });
});
