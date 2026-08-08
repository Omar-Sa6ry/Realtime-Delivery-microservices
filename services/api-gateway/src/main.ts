import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { waitForService } from './utils/waitService.util';

async function bootstrap() {
  // Wait for subgraph services to be available
  // await Promise.all([
  //   waitForService('http://user-srv:3000/user/graphql'),
  // ]);

  const app = await NestFactory.create(AppModule);
  app.enableCors();

  const port = process.env.PORT_GATEWAY || 4000;
  const server = await app.listen(port, '0.0.0.0');
  console.log(
    `API Gateway is running on: https://delivary.test/graphql or http://localhost:${port}/graphql`,
  );
}

bootstrap().catch((err) => {
  console.error('Bootstrap failed, retrying in 10s...', err.message);
  setTimeout(() => bootstrap(), 10000);
});
