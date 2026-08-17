import { Module } from '@nestjs/common';
import {
  WsGuardChain,
  WS_GUARD_CHAIN_OPTIONS,
  WS_GUARD_CHAIN_RATE_LIMITER,
} from '@delivery/common';
import { RealtimeGateway } from './websocket.gateway';
import { RATE_ACTIONS, VALIDATION } from './websocket.types';
import { RealtimeRateLimiterService } from '../rate-limit/realtime-rate-limiter.service';
import { ConnectionModule } from '../connection/connection.module';
import { SubscriptionModule } from '../subscription/subscription.module';
import { PresenceModule } from '../presence/presence.module';
import { HeartbeatModule } from '../heartbeat/heartbeat.module';
import { LocationModule } from '../location/location.module';
import { CommandModule } from '../command/command.module';
import { RateLimitModule } from '../rate-limit/rate-limit.module';
import { AuthorizationModule } from '../authorization/authorization.module';

@Module({
  imports: [
    ConnectionModule,
    SubscriptionModule,
    PresenceModule,
    HeartbeatModule,
    LocationModule,
    CommandModule,
    RateLimitModule,
    AuthorizationModule,
  ],
  providers: [
    RealtimeGateway,
    WsGuardChain,
    {
      provide: WS_GUARD_CHAIN_RATE_LIMITER,
      useExisting: RealtimeRateLimiterService,
    },
    {
      provide: WS_GUARD_CHAIN_OPTIONS,
      useValue: { validators: VALIDATION, rateActions: RATE_ACTIONS },
    },
  ],
  exports: [RealtimeGateway],
})
export class WebSocketModule {}