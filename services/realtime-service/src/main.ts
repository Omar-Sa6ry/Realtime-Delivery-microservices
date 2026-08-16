import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ConfigService } from '@nestjs/config';
import { StructuredLogger } from '@delivery/common';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, {
    logger: new StructuredLogger(),
  });

  app.enableCors();

  const configService = app.get(ConfigService);
  const port = configService.get<string>('PORT_REALTIME', '4006');

  await app.listen(port, '0.0.0.0');

  const logger = new StructuredLogger();
  logger.log(
    `Realtime Service is running on http://localhost:${port}/realtime/graphql`,
  );
}

bootstrap().catch((err) => {
  console.error('Bootstrap failed, retrying in 10s...', err.message);
  setTimeout(() => bootstrap(), 10000);
});