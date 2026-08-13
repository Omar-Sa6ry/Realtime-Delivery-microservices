/**
 * waitForService polls a GraphQL endpoint until it responds with valid data,
 * implementing an exponential back-off from `delay` ms up to `maxDelay` ms.
 *
 * @param url     - The GraphQL HTTP endpoint to probe.
 * @param retries - Maximum number of attempts before giving up (default 360 ≈ 30 min at 5 s).
 * @param delay   - Initial delay between retries in milliseconds (default 5000 ms).
 * @param maxDelay - Maximum delay between retries in milliseconds (default 30000 ms).
 */
export async function waitForService(
  url: string,
  retries = 360,
  delay = 5000,
  maxDelay = 30_000,
): Promise<void> {
  let currentDelay = delay;

  for (let i = 0; i < retries; i++) {
    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: '{ __typename }' }),
        // Abort if the service doesn't respond within 5 seconds
        signal: AbortSignal.timeout(5_000),
      });

      if (res.ok) {
        const body = await res.json() as Record<string, unknown>;
        const data = body?.data as Record<string, unknown> | undefined;
        if (data?.__typename) {
          console.log(`[startup] Service at ${url} is ready (attempt ${i + 1}).`);
          return;
        }
      }

      console.log(
        `[startup] Service at ${url} not ready yet (attempt ${i + 1}/${retries}, status ${res.status}). ` +
        `Retrying in ${Math.round(currentDelay / 1000)}s...`,
      );
    } catch {
      console.log(
        `[startup] Service at ${url} unreachable (attempt ${i + 1}/${retries}). ` +
        `Retrying in ${Math.round(currentDelay / 1000)}s...`,
      );
    }

    await new Promise<void>((r) => setTimeout(r, currentDelay));

    // Exponential back-off: double the delay each time, capped at maxDelay.
    currentDelay = Math.min(currentDelay * 1.5, maxDelay);
  }

  console.warn(
    `[startup] Service at ${url} not available after ${retries} attempts — starting API Gateway anyway.`,
  );
}