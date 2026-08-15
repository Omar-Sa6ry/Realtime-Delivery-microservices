import { Module } from '@nestjs/common';
import { ClientsModule, Transport } from '@nestjs/microservices';
import { join } from 'path';
import { USER_PACKAGE_NAME } from '@delivery/common';

@Module({
  imports: [
    ClientsModule.register([
      {
        name: 'GRPC_USER_SERVICE',
        transport: Transport.GRPC,
        options: {
          package: USER_PACKAGE_NAME,
          protoPath: join(process.cwd(), '../../protos/user.proto'),
          url: process.env.USER_GRPC_URL || 'user-srv:50051',
        },
      },
    ]),
  ],
  exports: [ClientsModule],
})
export class GrpcClientsModule {}