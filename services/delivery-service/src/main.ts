import { join } from 'path';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { I18nService, I18nValidationException } from 'nestjs-i18n';
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
  const i18n = app.get(I18nService);
  console.log(i18n.t('delivery.serviceInfo' as never) + ': http://0.0.0.0:' + port);

  // Start NATS + gRPC transports in the background after HTTP is live.
  app
    .startAllMicroservices()
    .catch((err: Error) =>
      logger.error(`Microservice startup error: ${err.message}`),
    );
}

bootstrap().catch((err) => {
  console.error(err.message);
  setTimeout(() => bootstrap(), 10000);
});






