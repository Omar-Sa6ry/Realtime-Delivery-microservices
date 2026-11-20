/**
 * Apollo Gateway `formatError` — normalises subgraph / gateway errors into a
 * single envelope. Extracted verbatim from the historical inline closure in
 * `app.module.ts` (behaviour preserved exactly).
 */
export function formatGatewayError(error: any): any {
  // Handle subgraph errors (new format: extensions directly on error)
  const subgraphError = error.extensions;

  if (
    subgraphError &&
    subgraphError.success === false &&
    subgraphError.statusCode
  ) {
    return {
      success: false,
      statusCode: subgraphError.statusCode,
      message: error.message,
      timeStamp: subgraphError.timeStamp || new Date().toISOString(),
      error: subgraphError.error,
    } as any;
  }

  // Handle legacy subgraph error format (if any)
  const legacySubgraphError = error.extensions?.response?.body?.errors?.[0];
  if (legacySubgraphError) {
    return {
      success: false,
      statusCode: legacySubgraphError.statusCode || 400,
      message: legacySubgraphError.message,
      timeStamp: legacySubgraphError.timeStamp || new Date().toISOString(),
    } as any;
  }

  // Handle gateway-level errors
  const originalError = error.extensions?.originalError as any;
  const msg = originalError?.message || error.message;
  const code = error.extensions?.statusCode || originalError?.statusCode || 400;

  return {
    success: false,
    statusCode: code,
    message: Array.isArray(msg) ? msg[0] : msg,
    timeStamp: new Date().toISOString(),
  } as any;
}
