import { Module } from '@nestjs/common';
import { UserService } from './user.service';
import { UserResolver } from './user.resolver';
import { UserGrpcController } from './user-grpc.controller';

@Module({
  controllers: [UserGrpcController],
  providers: [UserService, UserResolver],
  exports: [UserService],
})
export class UserModule {}
