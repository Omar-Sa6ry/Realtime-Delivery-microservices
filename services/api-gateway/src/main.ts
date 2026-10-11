import * as http from 'http';
import type { NestExpressApplication } from '@nestjs/platform-express';
import { StructuredLogger } from '@delivery-micro/shard';
import { waitForRedis, waitForService } from './utils/waitService.util';

const LIVENESS_PORT = Number(process.env.PORT_LIVENESS ?? 4099);

const logger = new StructuredLogger();

function startLivenessServer(): http.Server {
  const server = http.createServer((req, res) => {
    if (
      req.url === '/health' ||
      req.url === '/health/ready' ||
      req.url === '/health/live' ||
      req.url === '/'
    ) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'alive', service: 'api-gateway' }));
      return;
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'alive', service: 'api-gateway' }));
  });

  server.on('error', (err) => {
    logger.error(
      `[liveness] Server error: ${(err as NodeJS.ErrnoException).message}`,
      err.stack,
      'Liveness',
    );
  });

  server.listen(LIVENESS_PORT, '0.0.0.0', () => {
    logger.log(
      `[liveness] Health server listening on port ${LIVENESS_PORT}`,
      'Liveness',
    );
  });

  return server;
}

async function waitForDependencies() {
  await waitForRedis();

  const subgraphs = [
    'http://payment-srv:4002/payment/graphql',
    'http://realtime-srv:4006/realtime/graphql',
    'http://notification-srv:4004/notification/graphql',
    'http://media-srv:4005/media/graphql',
    'http://search-srv:4007/search/graphql',
    'http://user-srv:4001/user/graphql',
    'http://delivery-srv:4003/delivery/graphql',
    'http://driver-srv:4008/driver/graphql',
    'http://analytics-srv:4009/analytics/graphql',
  ];

  logger.log('[startup] Waiting for subgraphs to be available...', 'Startup');
  await Promise.all(subgraphs.map((url) => waitForService(url)));
  logger.log('All subgraphs are reachable.', 'Startup');
}

async function setupProxies(app: NestExpressApplication) {
  const { createProxyMiddleware } = await import('http-proxy-middleware');

  // WebSocket Proxy for Realtime Service
  const wsProxy = createProxyMiddleware({
    target: 'http://realtime-srv:4006',
    ws: true,
    changeOrigin: true,
    pathFilter: (pathname: string) =>
      pathname.startsWith('/realtime') || pathname.startsWith('/ws'),
    pathRewrite: { '^/realtime': '/ws' },
  });
  app.use(wsProxy);

  // HTTP Proxy for Localstack (S3)
  const s3Proxy = createProxyMiddleware({
    target: 'http://localstack-srv:4566',
    changeOrigin: true,
    pathFilter: '/localstack',
    pathRewrite: { '^/localstack': '' },
  });
  app.use(s3Proxy);

  return { wsProxy };
}

async function bootstrap() {
  await waitForDependencies();

  // Lazy-load AppModule and NestJS
  const { AppModule } = require('./app.module');
  const { NestFactory } = await import('@nestjs/core');

  const app = await NestFactory.create<NestExpressApplication>(AppModule, {
    logger,
  });

  app.set('trust proxy', 1);
  app.enableShutdownHooks();

  const helmet = (await import('helmet')).default;
  app.use(
    helmet({
      contentSecurityPolicy:
        process.env.NODE_ENV === 'production' ? undefined : false,
      crossOriginEmbedderPolicy: false,
    }),
  );

  app.enableCors({ origin: '*', credentials: true });

  const { wsProxy } = await setupProxies(app);

  const port = process.env.PORT_GATEWAY ?? 4000;
  await app.listen(port, '0.0.0.0');

  // Attach proxy upgrade handler
  const httpServer = app.getHttpServer();
  httpServer.on('upgrade', (req: any, socket: any, head: any) => {
    if (
      req.url &&
      (req.url.startsWith('/realtime') || req.url.startsWith('/ws'))
    ) {
      wsProxy.upgrade(req, socket, head);
    }
  });

  logger.log(
    `API Gateway is running on: https://delivery.test/graphql or http://localhost:${port}/graphql`,
    'Startup',
  );
}

function runBootstrap() {
  bootstrap().catch((err) => {
    logger.error(
      'Bootstrap failed, retrying in 10s...',
      err?.stack || err?.message || String(err),
      'Startup',
    );
    setTimeout(() => runBootstrap(), 10000);
  });
}

// ─── Process-level error guards ─────────────────────────────────────────────
process.on('unhandledRejection', (reason) => {
  const err = reason instanceof Error ? reason : new Error(String(reason));
  logger.error('Unhandled promise rejection', err.stack, 'Process');
});

process.on('uncaughtException', (err) => {
  logger.error(`Uncaught exception: ${err.message}`, err.stack, 'Process');
  process.exit(1);
});

// Start Liveness Server, then trigger Bootstrap
startLivenessServer();
runBootstrap();
