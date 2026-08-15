import { Module } from '@nestjs/common';
import { GrpcController } from './grpc.controller';
import { NotificationModule } from '../notification/notification.module';

@Module({
  imports: [NotificationModule],
  controllers: [GrpcController],
})
export class GrpcModule {}
