import { Global, Module } from '@nestjs/common';
import { RealtimeAuthorizationService } from './realtime-authorization.service';
import { DeliveryPolicy } from './policies/delivery.policy';
import { DriverPolicy } from './policies/driver.policy';
import { GrpcModule } from '../../infrastructure/grpc/grpc.module';

@Global()
@Module({
  imports: [GrpcModule],
  providers: [RealtimeAuthorizationService, DeliveryPolicy, DriverPolicy],
  exports: [RealtimeAuthorizationService, DeliveryPolicy, DriverPolicy],
})
export class AuthorizationModule {}