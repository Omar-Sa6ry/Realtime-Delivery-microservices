import { Global, Module } from '@nestjs/common';
import { GrpcClient } from './grpc.client';

@Global()
@Module({
  providers: [GrpcClient],
  exports: [GrpcClient],
})
export class GrpcModule {}