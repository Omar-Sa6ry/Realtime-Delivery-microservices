import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { MicroserviceOptions, Transport } from '@nestjs/microservices';
import { ConfigService } from '@nestjs/config';
import { StructuredLogger } from '@delivery/common';
import { join } from 'path';
import { I18nValidationException } from 'nestjs-i18n';

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

  const configService = app.get(ConfigService);

  // gRPC Microservice
  app.connectMicroservice<MicroserviceOptions>({
    transport: Transport.GRPC,
    options: {
      package: 'notification',
      protoPath: join(process.cwd(), '../../protos/notification.proto'),
      url: `0.0.0.0:${configService.get<string>('PORT_GRPC', '50053')}`,
    },
  });

  const port = configService.get<string>('PORT_NOTIFICATION', '4004');
  await app.listen(port, '0.0.0.0');

  logger.log(`Notification Service is running on http://localhost:${port}/notification/graphql`);

  // Start gRPC transport in the background after HTTP is live.
  app.startAllMicroservices()
    .then(() =>
      logger.log(`gRPC Server is running on port ${configService.get('PORT_GRPC', '50053')}`),
    )
    .catch((err: Error) =>
      logger.error(`Microservice startup error: ${err.message}`),
    );
}

// Keep the process alive while infrastructure (Redis, DB, NATS, Kafka) is still starting:
process.on('uncaughtException', (err) => {
  console.error('[process] Uncaught exception (continuing):', err?.message ?? err);
});
process.on('unhandledRejection', (reason) => {
  console.error('[process] Unhandled rejection (continuing):', reason);
});

bootstrap().catch((err) => {
  console.error('Bootstrap failed, retrying in 10s...', err.message);
  setTimeout(() => bootstrap(), 10000);
});