import { Global, Module } from '@nestjs/common';
import { LocationService } from './location.service';
import { LocationValidator } from './location.validator';
import { LocationThrottler } from './location.throttler';
import { AuthorizationModule } from '../authorization/authorization.module';
import { ConnectionModule } from '../connection/connection.module';
import { RateLimitModule } from '../rate-limit/rate-limit.module';

@Global()
@Module({
  imports: [AuthorizationModule, ConnectionModule, RateLimitModule],
  providers: [LocationService, LocationValidator, LocationThrottler],
  exports: [LocationService],
})
export class LocationModule {}