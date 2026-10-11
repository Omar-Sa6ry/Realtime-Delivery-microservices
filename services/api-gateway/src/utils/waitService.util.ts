import * as net from 'net';
import { StructuredLogger } from '@delivery-micro/shard';

const logger = new StructuredLogger();

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
      const response = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: '{ __typename }' }),
        signal: AbortSignal.timeout(5_000),
      });

      const body = (await response.json()) as {
        data?: { __typename?: string };
      };

      if (response.ok && body.data?.__typename) {
        logger.log(
          `[startup] ${url} is ready (attempt ${attempt}).`,
          'Startup',
        );
        return;
      }

      logger.warn(
        `[startup] ${url} is not ready. Retrying...`,
        'Startup',
      );
    } catch (err) {
      logger.warn(
        `[startup] ${url} is unreachable (${(err as Error).message}). Retrying...`,
        'Startup',
      );
    }

    if (signal?.aborted) {
      logger.warn(`[startup] Waiting for ${url} was cancelled.`, 'Startup');
      return;
    }

    await sleep(currentDelay);
    currentDelay = Math.min(currentDelay * 1.5, maxDelay);
  }
}

export async function waitForRedis(
  host = process.env.REDIS_HOST ?? 'redis-srv',
  port = Number(process.env.REDIS_PORT ?? 6379),
): Promise<void> {
  for (;;) {
    try {
      await canConnect(host, port);
      logger.log(`[startup] Redis is ready at ${host}:${port}.`, 'Startup');
      return;
    } catch (err) {
      logger.log(
        `[startup] Redis is not ready (${(err as Error).message}). Retrying in 2s...`,
        'Startup',
      );
      await sleep(2000);
    }
  }
}

function canConnect(host: string, port: number): Promise<void> {
  return new Promise((resolve, reject) => {
    const socket = net.createConnection({ host, port });

    socket.once('connect', () => {
      socket.destroy();
      resolve();
    });

    socket.once('error', (error) => {
      socket.destroy();
      reject(error);
    });

    socket.setTimeout(2000, () => {
      socket.destroy();
      reject(new Error('Connection timed out'));
    });
  });
}

function sleep(milliseconds: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}
