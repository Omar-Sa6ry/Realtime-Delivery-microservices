import { join } from 'path';
import { Global, Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { ClientsModule, Transport } from '@nestjs/microservices';
import { GrpcServer } from './grpc.server';

@Global()
@Module({
  imports: [
    ConfigModule,
    ClientsModule.registerAsync([
      {
        name: 'DRIVER_SERVICE',
        imports: [ConfigModule],
        useFactory: (configService: ConfigService) => ({
          transport: Transport.GRPC,
          options: {
            package: 'driver',
            protoPath: join(process.cwd(), '../../protos/driver.proto'),
            url: configService.get<string>('DRIVER_SERVICE_URL') || 'driver-srv:50055',
            loader: {
              keepCase: true,
              longs: String,
              enums: String,
              defaults: true,
              oneofs: true,
            },
          },
        }),
        inject: [ConfigService],
      },
    ]),
  ],
  providers: [GrpcServer],
  exports: [GrpcServer],
})
export class DeliveryGrpcModule {}