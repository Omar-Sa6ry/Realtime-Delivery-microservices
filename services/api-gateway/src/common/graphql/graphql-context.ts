/**
 * Decodes (WITHOUT verifying) a base64 JWT payload and attaches it to `req.user`.
 *
 * NOTE: this mirrors the historical gateway behaviour — the GraphQL context
 * only *decodes* the token so subgraphs receive identity headers; signature /
 * expiry verification is delegated to the subgraphs. It intentionally does NOT
 * throw on a malformed token (it just leaves the request anonymous).
 */
export function applyUserFromToken(req: any): void {
  if (!req || req.user) return;
  if (!req.headers?.authorization?.startsWith('Bearer ')) return;

  try {
    const token = req.headers.authorization.split(' ')[1];
    const parts = token.split('.');
    if (parts.length !== 3) return;

    const payload = JSON.parse(
      Buffer.from(parts[1], 'base64').toString('utf-8'),
    );
    req.user = {
      userId: payload.sub || payload.userId || payload.id,
      role: payload.role || 'USER',
      sessionId: payload.sessionId,
    };
  } catch {
    // ignore invalid format
  }
}

/**
 * The Apollo Gateway `context` factory: ensures `req.user` is populated from a
 * decoded (unverified) Bearer token and returns `{ req }`.
 */
export function buildGraphQLContext({ req }: { req?: any }): { req?: any } {
  applyUserFromToken(req);
  return { req };
}
