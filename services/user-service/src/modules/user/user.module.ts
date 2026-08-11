import { Module } from '@nestjs/common';
import { UserService } from './user.service';
import { UserResolver } from './user.resolver';
import { UserGrpcController } from './user-grpc.controller';
import { MetricsController } from './metrics.controller';
import { HealthController } from './health.controller';

@Module({
  controllers: [UserGrpcController, MetricsController, HealthController],
  providers: [UserService, UserResolver],
  exports: [UserService],
})
export class UserModule {}
