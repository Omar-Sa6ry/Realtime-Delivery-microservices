import { Global, Module } from '@nestjs/common';
import { DeliveryModule } from '../../delivery/delivery.module';
import { GrpcServer } from './grpc.server';

@Global()
@Module({
  imports: [DeliveryModule],
  providers: [GrpcServer],
  exports: [GrpcServer],
})
export class DeliveryGrpcModule {}
