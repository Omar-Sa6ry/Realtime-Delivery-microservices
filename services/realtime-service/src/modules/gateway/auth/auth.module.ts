import { Global, Module } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import { WsAuthGuard, WsJwtStrategy, WS_JWT_SERVICE } from '@delivery/common';

@Global()
@Module({
  providers: [
    { provide: WS_JWT_SERVICE, useExisting: JwtService },
    WsAuthGuard,
    WsJwtStrategy,
  ],
  exports: [WsAuthGuard, WsJwtStrategy],
})
export class AuthModule {}