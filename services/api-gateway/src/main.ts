import * as http from 'http';
import { NestFactory } from '@nestjs/core';
import { NestExpressApplication } from '@nestjs/platform-express';
import { AppModule } from './app.module';
import helmet from 'helmet';
import { StructuredLogger } from '@delivery/common';
import { waitForService } from './utils/waitService.util';

const LIVENESS_PORT = Number(process.env.PORT_LIVENESS ?? 4099);
const livenessServer = http.createServer((_req, res) => {
  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ status: 'alive', service: 'api-gateway' }));
});

livenessServer.on('error', (err) =>
  console.error(
    '[liveness] Server error:',
    (err as NodeJS.ErrnoException).message,
  ),
);

livenessServer.listen(LIVENESS_PORT, '0.0.0.0', () =>
  console.log(`[liveness] Health server listening on port ${LIVENESS_PORT}`),
);

async function bootstrap() {
  const logger = new StructuredLogger();

  const app = await NestFactory.create<NestExpressApplication>(AppModule, {
    logger,
  });

  // Trust Proxy is crucial for Rate Limiting behind Load Balancers / Ingress
  app.set('trust proxy', 1);

  // Enable graceful shutdown hooks
  app.enableShutdownHooks();

  // Security Hardening
  app.use(
    helmet({
      contentSecurityPolicy:
        process.env.NODE_ENV === 'production' ? undefined : false,
      crossOriginEmbedderPolicy: false,
    }),
  );

  app.enableCors({
    origin: '*',
    credentials: true,
  });

  const port = process.env.PORT_GATEWAY ?? 4000;
  await app.listen(port, '0.0.0.0');

  await Promise.all([
    waitForService('http://realtime-srv:4006/realtime/graphql'),
    waitForService('http://notification-srv:4004/notification/graphql'),
    waitForService('http://media-srv:4005/media/graphql'),
    waitForService('http://search-srv:4007/search/graphql'),
    waitForService('http://user-srv:4001/user/graphql'),
  ])
    .then(() => {
      logger.log('All subgraphs are reachable.');
      logger.log(
        `API Gateway is running on: https://delivary.test/graphql or http://localhost:${port}/graphql`,
      );
    })
    .catch((err: Error) => logger.warn(`Subgraph wait error: ${err.message}`));
}

function runBootstrap() {
  bootstrap().catch((err) => {
    console.error('Bootstrap failed, retrying in 10s...', err.message);
    setTimeout(() => runBootstrap(), 10000);
  });
}

runBootstrap();
