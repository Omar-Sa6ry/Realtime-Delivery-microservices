import { join } from 'path';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import { I18nValidationException } from 'nestjs-i18n';
import { MicroserviceOptions, Transport } from '@nestjs/microservices';
import { graphqlUploadExpress } from 'graphql-upload-minimal';
import { USER_PACKAGE_NAME, StructuredLogger } from '@delivery-micro/shard';

const logger = new StructuredLogger();

/** HTTP port of the service, overridable through `PORT_USER`. */
export const DEFAULT_PORT = 4001;

/** NATS endpoint used when `NATS_URL` is not configured. */
export const DEFAULT_NATS_URL = 'nats://localhost:4222';

/** gRPC endpoint the federation reference resolver listens on. */
export const GRPC_URL = '0.0.0.0:50051';

/** Delay before a failed bootstrap is retried. */
export const BOOTSTRAP_RETRY_MS = 10000;

/**
 * Configures the HTTP pipeline (CORS, uploads, validation) and attaches the
 * NATS + gRPC transports. Exported so the unit tests can assert the wiring
 * without binding a port.
 */
export function configureApp(app: INestApplication): void {
  app.enableCors();

  app.use(graphqlUploadExpress({ maxFileSize: 100_000_000, maxFiles: 5 }));

  app.useGlobalPipes(
    new ValidationPipe({
      transform: true,
      whitelist: true,
      stopAtFirstError: true,
      exceptionFactory: (errors) => {
        return new I18nValidationException(errors);
      },
    }),
  );

  // NATS Microservice Transport
  app.connectMicroservice<MicroserviceOptions>({
    transport: Transport.NATS,
    options: {
      servers: [process.env.NATS_URL || DEFAULT_NATS_URL],
      queue: 'user-service',
      timeout: 5000,
      reconnectTimeWait: 2000,
    },
  });

  // gRPC Microservice Transport
  app.connectMicroservice<MicroserviceOptions>({
    transport: Transport.GRPC,
    options: {
      package: USER_PACKAGE_NAME,
      protoPath: join(process.cwd(), '../../protos/user.proto'),
      url: GRPC_URL,
      loader: {
        keepCase: true,
      },
    },
  });
}

/** Resolves the HTTP port, falling back to the documented default. */
export function resolvePort(
  raw: string | undefined = process.env.PORT_USER,
): number {
  const port = Number(raw);

  return Number.isFinite(port) && port > 0 ? port : DEFAULT_PORT;
}

export async function bootstrap(): Promise<void> {
  const app = await NestFactory.create(AppModule, {
    rawBody: true,
    logger,
    abortOnError: false,
  });

  configureApp(app);

  const port = resolvePort();
  await app.listen(port, '0.0.0.0');
  console.log(`User Service is running on http://0.0.0.0:${port}`);

  // Start NATS + gRPC transports in the background after HTTP is live.
  app
    .startAllMicroservices()
    .catch((err: Error) =>
      logger.error(`Microservice startup error: ${err.message}`),
    );
}

/**
 * Retries the bootstrap forever: infrastructure (Redis, DB, NATS) may still
 * be starting when the pod comes up, and library-level errors (e.g. an ioredis
 * connection refusal) must not kill the process.
 */
export function runBootstrap(
  attempt: () => Promise<void> = bootstrap,
  retryMs: number = BOOTSTRAP_RETRY_MS,
): void {
  attempt().catch((err: Error) => {
    console.error('Bootstrap failed, retrying in 10s...', err.message);
    setTimeout(() => runBootstrap(attempt, retryMs), retryMs);
  });
}

/**
 * Keeps the process alive while infrastructure is still starting: a
 * library-level error must not kill the pod — the bootstrap retry loop
 * recovers once dependencies are reachable.
 */
export function registerProcessHandlers(
  target: NodeJS.Process = process,
): void {
  target.on('uncaughtException', (err: Error) => {
    console.error(
      '[process] Uncaught exception (continuing):',
      err?.message ?? err,
    );
  });
  target.on('unhandledRejection', (reason: unknown) => {
    console.error('[process] Unhandled rejection (continuing):', reason);
  });
}

/** The startup actions `runIfEntryPoint` triggers when this file is the entry point. */
export interface EntryPointActions {
  registerProcessHandlers: () => void;
  runBootstrap: () => void;
}

/**
 * Boot the service only when this file is the process entry point
 * (`node dist/main`). When imported by the unit tests we skip the side
 * effects so the module can be exercised without starting servers.
 *
 * The predicate and actions are injectable (defaulting to the real
 * behaviour) purely so tests can cover the entry-point branch.
 */
export function runIfEntryPoint(
  isEntryPoint: boolean = typeof require !== 'undefined' &&
    require.main === module,
  actions: EntryPointActions = {
    registerProcessHandlers,
    runBootstrap,
  },
): void {
  if (isEntryPoint) {
    actions.registerProcessHandlers();
    actions.runBootstrap();
  }
}

runIfEntryPoint();
