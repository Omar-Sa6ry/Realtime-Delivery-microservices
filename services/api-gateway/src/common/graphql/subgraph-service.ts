import { RemoteGraphQLDataSource } from '@apollo/gateway';
import { CORRELATION_ID_HEADER } from '../middlewares/correlation-id.middleware';

/**
 * `willSendRequest` for a subgraph data source — forwards identity, correlation
 * and language headers from the incoming gateway request to the subgraph.
 * Extracted verbatim from the historical inline closure in `app.module.ts`.
 */
export function willSendRequest({
  request,
  context,
}: {
  request: any;
  context: any;
}): void {
  let user = context.req?.user;

  // Fallback: decode (unverified) the Bearer token if the context user is absent.
  if (!user && context.req?.headers?.authorization?.startsWith('Bearer ')) {
    try {
      const token = context.req.headers.authorization.split(' ')[1];
      const parts = token.split('.');
      if (parts.length === 3) {
        const payload = JSON.parse(
          Buffer.from(parts[1], 'base64').toString('utf-8'),
        );
        user = {
          userId: payload.sub || payload.userId || payload.id,
          role: payload.role || 'USER',
          sessionId: payload.sessionId,
        };
      }
    } catch {
      // ignore
    }
  }

  if (user) {
    request.http.headers.set('x-user-id', user.userId || '');
    request.http.headers.set('x-user-role', user.role || '');
    if (user.sessionId) {
      request.http.headers.set('x-user-session', user.sessionId);
    }
  }
  if (context.req?.headers?.[CORRELATION_ID_HEADER]) {
    request.http.headers.set(
      CORRELATION_ID_HEADER,
      context.req.headers[CORRELATION_ID_HEADER],
    );
  }
  if (context.req?.headers?.authorization) {
    request.http.headers.set(
      'authorization',
      context.req.headers.authorization,
    );
  }
  const lang =
    context.req?.headers?.['x-lang'] ||
    context.req?.headers?.['accept-language'];
  if (lang) {
    request.http.headers.set('x-lang', lang);
    request.http.headers.set('accept-language', lang);
  }
}

/**
 * `buildService` for the Apollo Gateway — returns a RemoteGraphQLDataSource for
 * a subgraph URL with header-forwarding wired in.
 */
export function buildSubgraphService({ url }: { url?: string }) {
  return new RemoteGraphQLDataSource({
    url,
    willSendRequest,
  });
}

/** The gateway's `buildService` implementation (delegates to the helper above). */
export const buildService = buildSubgraphService;
