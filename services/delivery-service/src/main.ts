import { join } from 'path';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { I18nService, I18nValidationException } from 'nestjs-i18n';
import { MicroserviceOptions, Transport } from '@nestjs/microservices';
import { StructuredLogger } from '@delivery/common';

async function bootstrap() {
  const logger = new StructuredLogger();
  let app;

  try {
    app = await NestFactory.create(AppModule, {
      rawBody: true,
      logger,
      abortOnError: false,
    });
    app.enableCors();

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

    // // NATS Microservice Transport
    app.connectMicroservice({
      transport: Transport.NATS,
      options: {
        servers: [process.env.NATS_URL || 'nats://localhost:4222'],
        queue: 'delivery-service',
        timeout: 5000,
        reconnectTimeWait: 2000,
      },
    });

    // // gRPC Microservice Transport
    app.connectMicroservice({
      transport: Transport.GRPC,
      options: {
        package: 'delivery',
        protoPath: join(process.cwd(), '../../protos/delivery.proto'),
        url: '0.0.0.0:' + (process.env.PORT_DELIVERY_GRPC ?? '50054'),
        loader: { keepCase: false },
      },
    });

    const port = Number(process.env.PORT_DELIVERY ?? 4003);
    process.env.PORT_DELIVERY_GRPC ??= '50054';
    process.env.PORT_METRICS ??= '9104';

    await app.listen(port, '0.0.0.0');
    const i18n = app.get(I18nService);
    console.log(
      i18n.t('delivery.serviceInfo' as never) + ': http://0.0.0.0:' + port,
    );

    // Start NATS + gRPC transports in the background after HTTP is live.
    app
      .startAllMicroservices()
      .catch((err: Error) =>
        logger.error(`Microservice startup error: ${err.message}`),
      );
  } catch (err) {
    console.error('NestJS bootstrap error:', err?.message ?? err);
    throw err;
  }
}

// Keep the process alive while infrastructure (Redis, DB, NATS) is still starting:
// library-level errors (e.g. ioredis connection) must not kill the pod — the bootstrap retry loop recovers once dependencies are reachable.
process.on('uncaughtException', (err) => {
  console.error('[process] Uncaught exception (continuing):', err?.message ?? err);
});
process.on('unhandledRejection', (reason) => {
  console.error('[process] Unhandled rejection (continuing):', reason);
});

// Wait for dependencies to be ready before starting the app
const maxRetries = 30;
const retryDelay = 2000;

async function waitForDependencies(): Promise<void> {
  const dependencies = [
    { name: 'PostgreSQL', check: async () => {
      try {
        const { Pool } = require('pg');
        const pool = new Pool({
          host: process.env.DB_HOST || 'delivery-db-srv',
          port: parseInt(process.env.DB_PORT || '5432'),
          user: process.env.DB_USERNAME || 'postgres',
          password: process.env.POSTGRES_PASSWORD || 'postgres',
          database: process.env.DB_NAME || 'delivery_delivery_db',
        });
        await pool.query('SELECT 1');
        await pool.end();
        return true;
      } catch {
        return false;
      }
    }},
    { name: 'Redis', check: async () => {
      try {
        const net = require('net');
        const client = net.createConnection({ port: parseInt(process.env.REDIS_PORT || '6379'), host: process.env.REDIS_HOST || 'redis-srv' });
        return new Promise((resolve) => {
          client.on('connect', () => { client.destroy(); resolve(true); });
          client.on('error', () => { client.destroy(); resolve(false); });
          client.on('timeout', () => { client.destroy(); resolve(false); });
        });
      } catch {
        return false;
      }
    }},
    { name: 'NATS', check: async () => {
      try {
        const net = require('net');
        const client = net.createConnection({ port: 4222, host: 'nats-srv' });
        return new Promise((resolve) => {
          client.on('connect', () => { client.destroy(); resolve(true); });
          client.on('error', () => { client.destroy(); resolve(false); });
          client.on('timeout', () => { client.destroy(); resolve(false); });
        });
      } catch {
        return false;
      }
    }},
    { name: 'Kafka', check: async () => {
      try {
        const { Kafka } = require('kafkajs');
        const kafka = new Kafka({ clientId: 'delivery-service', brokers: [process.env.KAFKA_BROKERS || 'kafka-srv:9092'] });
        const admin = kafka.admin();
        await admin.connect();
        await admin.listTopics();
        await admin.disconnect();
        return true;
      } catch {
        return false;
      }
    }},
  ];

  for (const dep of dependencies) {
    let retries = 0;
    while (retries < maxRetries) {
      if (await dep.check()) {
        console.log(`${dep.name} is ready`);
        break;
      }
      retries++;
      console.log(`${dep.name} not ready, retrying... (${retries}/${maxRetries})`);
      await new Promise(r => setTimeout(r, retryDelay));
    }
    if (retries >= maxRetries) {
      console.error(`${dep.name} failed to become ready after ${maxRetries} attempts`);
    }
  }
}

// Wait for dependencies then start the application
waitForDependencies().then(() => {
  bootstrap().catch((err) => {
    console.error('Bootstrap failed, retrying in 10s...', err.message);
    setTimeout(() => bootstrap(), 10000);
  });
}).catch((err) => {
  console.error('Dependency wait failed:', err);
  bootstrap().catch((err) => {
    console.error('Bootstrap failed, retrying in 10s...', err.message);
    setTimeout(() => bootstrap(), 10000);
  });
});