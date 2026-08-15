import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';

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
       timeout: 5000,
       reconnectTimeWait: 2000,
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
   const port = process.env.PORT_NOTIFICATION ?? 4004;
   await app.listen(port, '0.0.0.0');
   console.log(`Notification Service is running on http://0.0.0.0:${port}`);
 }
 
 bootstrap().catch((err) => {
   console.error('Bootstrap failed, retrying in 10s...', err.message);
   setTimeout(() => bootstrap(), 10000);
 });