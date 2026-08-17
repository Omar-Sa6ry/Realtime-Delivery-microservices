import { Global, Module } from '@nestjs/common';
import { SubscriptionService } from './subscription.service';
import { SubscriptionStore } from './subscription.store';
import { AuthorizationModule } from '../authorization/authorization.module';

@Global()
@Module({
  imports: [AuthorizationModule],
  providers: [SubscriptionService, SubscriptionStore],
  exports: [SubscriptionService, SubscriptionStore],
})
export class SubscriptionModule {}