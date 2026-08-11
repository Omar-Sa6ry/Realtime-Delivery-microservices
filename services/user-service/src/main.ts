import { join } from 'path';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { setupInterceptors } from '@bts-soft/core';
import { I18nValidationException } from 'nestjs-i18n';
import { MicroserviceOptions, Transport } from '@nestjs/microservices';
import { graphqlUploadExpress } from 'graphql-upload-minimal';
import { USER_PACKAGE_NAME, StructuredLogger } from '@delivery/common';

async function bootstrap() {
  const logger = new StructuredLogger();
  const app = await NestFactory.create(AppModule, { 
    rawBody: true,
    logger,
  });
  app.enableCors();

  app.use(graphqlUploadExpress({ maxFileSize: 100_000_000, maxFiles: 5 }));
  setupInterceptors(app as any);

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
      servers: [process.env.NATS_URL || 'nats://localhost:4222'],
      queue: 'user-service',
    },
  });

  // gRPC Microservice Transport
  app.connectMicroservice<MicroserviceOptions>({
    transport: Transport.GRPC,
    options: {
      package: USER_PACKAGE_NAME,
      protoPath: join(process.cwd(), '../../protos/user.proto'),
      url: '0.0.0.0:50051',
      loader: {
        keepCase: true,
      },
    },
  });

  await app.startAllMicroservices();
  const port = process.env.PORT_USER ?? 4001;
  await app.listen(port, '0.0.0.0');
  console.log(`User Service is running on http://0.0.0.0:${port}`);
}

bootstrap();