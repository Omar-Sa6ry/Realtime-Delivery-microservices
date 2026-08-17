import { Global, Module } from '@nestjs/common';
import { LocationService } from './location.service';
import { LocationValidator } from './location.validator';
import { LocationThrottler } from './location.throttler';
import { AuthorizationModule } from '../../gateway/authorization/authorization.module';
import { ConnectionModule } from '../../gateway/connection/connection.module';

@Global()
@Module({
  imports: [AuthorizationModule, ConnectionModule],
  providers: [LocationService, LocationValidator, LocationThrottler],
  exports: [LocationService],
})
export class LocationModule {}