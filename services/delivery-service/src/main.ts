import { join } from 'path';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { I18nValidationException } from 'nestjs-i18n';
import { MicroserviceOptions, Transport } from '@nestjs/microservices';
import { StructuredLogger } from '@delivery/common';

async function bootstrap() {
  const logger = new StructuredLogger();
  const app = await NestFactory.create(AppModule, {
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
  app.connectMicroservice<MicroserviceOptions>({
    transport: Transport.NATS,
    options: {
      servers: [process.env.NATS_URL || 'nats://localhost:4222'],
      queue: 'delivery-service',
      timeout: 5000,
      reconnectTimeWait: 2000,
    },
  });

  // // gRPC Microservice Transport
  app.connectMicroservice<MicroserviceOptions>({
    transport: Transport.GRPC,
    options: {
      package: 'delivery',
      protoPath: join(process.cwd(), '../../protos/delivery.proto'),
      url: '0.0.0.0:' + (process.env.PORT_DELIVERY_GRPC ?? '50054'),
      loader: { keepCase: false },
    },
  });

  app.connectMicroservice<MicroserviceOptions>({
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
  console.log(`Delivery Service is running on http://0.0.0.0:${port}`);

  // Start NATS + gRPC transports in the background after HTTP is live.
  app
    .startAllMicroservices()
    .catch((err: Error) =>
      logger.error(`Microservice startup error: ${err.message}`),
    );
}

bootstrap().catch((err) => {
  console.error('Bootstrap failed, retrying in 10s...', err.message);
  setTimeout(() => bootstrap(), 10000);
});
