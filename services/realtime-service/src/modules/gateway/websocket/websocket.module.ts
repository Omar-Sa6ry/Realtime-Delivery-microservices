import { Module } from '@nestjs/common';
import {
  WsGuardChain,
  WS_GUARD_CHAIN_OPTIONS,
  WS_GUARD_CHAIN_RATE_LIMITER,
} from '@delivery/common';
import { RealtimeGateway } from './websocket.gateway';
import { RATE_ACTIONS, VALIDATION } from './websocket.types';
import { WebsocketRateLimiterService } from './websocket-rate-limiter.service';
import { ConnectionModule } from '../connection/connection.module';
import { SubscriptionModule } from '../../features/subscription/subscription.module';
import { PresenceModule } from '../../features/presence/presence.module';
import { HeartbeatModule } from '../heartbeat/heartbeat.module';
import { LocationModule } from '../../features/location/location.module';
import { CommandModule } from '../../features/command/command.module';
import { AuthorizationModule } from '../authorization/authorization.module';

@Module({
  imports: [
    ConnectionModule,
    SubscriptionModule,
    PresenceModule,
    HeartbeatModule,
    LocationModule,
    CommandModule,
    AuthorizationModule,
  ],
  providers: [
    RealtimeGateway,
    WsGuardChain,
    WebsocketRateLimiterService,
    {
      provide: WS_GUARD_CHAIN_RATE_LIMITER,
      useExisting: WebsocketRateLimiterService,
    },
    {
      provide: WS_GUARD_CHAIN_OPTIONS,
      useValue: { validators: VALIDATION, rateActions: RATE_ACTIONS },
    },
  ],
  exports: [RealtimeGateway],
})
export class WebSocketModule {}