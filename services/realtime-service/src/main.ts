import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ValidationPipe } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { StructuredLogger } from '@delivery/common';
import { I18nValidationException } from 'nestjs-i18n';
import { RealtimeWsAdapter } from '@delivery/common';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, {
    rawBody: true,
    logger: new StructuredLogger(),
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

  const maxPayload =
    Number(configService.get<string>('WS_MAX_PAYLOAD', '16384')) || 16384;
  app.useWebSocketAdapter(new RealtimeWsAdapter(app.getHttpServer(), { maxPayload }));

  const port = configService.get<string>('PORT_REALTIME', '4006');
  await app.listen(port, '0.0.0.0');

  const logger = new StructuredLogger();
  logger.log(
    `Realtime Service is running on http://localhost:${port}/realtime/graphql`,
  );

  const graceful = async (signal: string) => {
    logger.log(`Received ${signal} — shutting down gracefully...`);
    await app.close();
    process.exit(0);
  };
  process.on('SIGTERM', () => graceful('SIGTERM'));
  process.on('SIGINT', () => graceful('SIGINT'));
}

bootstrap().catch((err) => {
  setTimeout(() => bootstrap(), 10000);
});
