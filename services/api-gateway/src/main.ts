import { NestFactory } from '@nestjs/core';
import { NestExpressApplication } from '@nestjs/platform-express';
import { AppModule } from './app.module';
import helmet from 'helmet';

async function bootstrap() {
  const app = await NestFactory.create<NestExpressApplication>(AppModule);
  
  // Trust Proxy is crucial for Rate Limiting behind Load Balancers / Ingress
  app.set('trust proxy', 1);

  // Enable graceful shutdown hooks
  app.enableShutdownHooks();

  // Security Hardening
  app.use(
    helmet({
      contentSecurityPolicy: process.env.NODE_ENV === 'production' ? undefined : false,
      crossOriginEmbedderPolicy: false,
    }),
  );

  app.enableCors({
    origin: '*',
    credentials: true,
  });

  const port = process.env.PORT_GATEWAY || 4000;
  await app.listen(port, '0.0.0.0');
  console.log(
    `API Gateway is running on: https://delivary.test/graphql or http://localhost:${port}/graphql`,
  );
}

bootstrap().catch((err) => {
  console.error('Bootstrap failed, retrying in 10s...', err.message);
  setTimeout(() => bootstrap(), 10000);
});
