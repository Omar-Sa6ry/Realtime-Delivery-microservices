export async function waitForService(
  url: string,
  signal?: AbortSignal,
  delay = 5000,
  maxDelay = 30_000,
): Promise<void> {
  let currentDelay = delay;
  let attempt = 0;

  for (;;) {
    attempt += 1;
    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: '{ __typename }' }),
        // Abort if the service doesn't respond within 5 seconds
        signal: AbortSignal.timeout(5_000),
      });

      if (res.ok) {
        const body = (await res.json()) as Record<string, unknown>;
        const data = body?.data as Record<string, unknown> | undefined;
        if (data?.__typename) {
          console.log(
            `[startup] Service at ${url} is ready (attempt ${attempt}).`,
          );
          return;
        }
      }

      console.log(
        `[startup] Service at ${url} not ready yet (attempt ${attempt}, status ${res.status}). ` +
          `Retrying in ${Math.round(currentDelay / 1000)}s...`,
      );
    } catch {
      console.log(
        `[startup] Service at ${url} unreachable (attempt ${attempt}). ` +
          `Retrying in ${Math.round(currentDelay / 1000)}s...`,
      );
    }

    if (signal?.aborted) {
      console.warn(
        `[startup] Wait for service at ${url} aborted — continuing anyway.`,
      );
      return;
    }

    await new Promise<void>((r) => setTimeout(r, currentDelay));

    currentDelay = Math.min(currentDelay * 1.5, maxDelay);
  }
}