import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { MicroserviceOptions, Transport } from '@nestjs/microservices';
import { ConfigService } from '@nestjs/config';
import { StructuredLogger } from '@delivery/common';
import { join } from 'path';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, {
    logger: ['error', 'warn', 'log', 'debug'],
  });

  const configService = app.get(ConfigService);
  const logger = new StructuredLogger();
  app.useLogger(logger);

  // gRPC Microservice
  app.connectMicroservice<MicroserviceOptions>({
    transport: Transport.GRPC,
    options: {
      package: 'notification',
      protoPath: join(__dirname, '../../../protos/notification.proto'),
      url: `0.0.0.0:${configService.get<string>('PORT_GRPC', '50053')}`,
    },
  });

  await app.startAllMicroservices();

  const port = configService.get<string>('PORT_NOTIFICATION', '4004');
  await app.listen(port);
  
  logger.log(`Notification Service is running on http://localhost:${port}/notification/graphql`);
  logger.log(`gRPC Server is running on port ${configService.get('PORT_GRPC', '50053')}`);
}
bootstrap();