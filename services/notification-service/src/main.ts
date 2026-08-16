import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { MicroserviceOptions, Transport } from '@nestjs/microservices';
import { ConfigService } from '@nestjs/config';
import { StructuredLogger } from '@delivery/common';
import { setupInterceptors } from '@bts-soft/core';
import { join } from 'path';
import { I18nValidationException } from 'nestjs-i18n';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, {
    rawBody: true,
    logger: new StructuredLogger(),
  });

  app.enableCors();
  setupInterceptors(app);

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
      protoPath: join(__dirname, '../../../protos/notification.proto'),
      url: `0.0.0.0:${configService.get<string>('PORT_GRPC', '50053')}`,
    },
  });

  const port = configService.get<string>('PORT_NOTIFICATION', '4004');
  await app.listen(port, '0.0.0.0');

  const logger = new StructuredLogger();
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

bootstrap().catch((err) => {
  console.error('Bootstrap failed, retrying in 10s...', err.message);
  setTimeout(() => bootstrap(), 10000);
});