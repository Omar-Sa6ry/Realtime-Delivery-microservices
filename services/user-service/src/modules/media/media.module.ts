import { Global, Module } from '@nestjs/common';
import { MediaGrpcService } from './grpc-media.service';

@Global()
@Module({
  providers: [MediaGrpcService],
  exports: [MediaGrpcService],
})
export class MediaModule {}
