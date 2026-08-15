import { Global, Module } from '@nestjs/common';
import { RoleGuard } from '@delivery/common';
import { JwtModule } from '@nestjs/jwt';
import { RedisModule } from '@bts-soft/cache';
import { GrpcClientsModule } from './grpc-clients.module';
import { UserLookupService } from './user-lookup.service';

@Global()
@Module({
  imports: [RedisModule, GrpcClientsModule, JwtModule],
  providers: [
    UserLookupService,
    RoleGuard,
    {
      provide: 'USER_SERVICE',
      useExisting: UserLookupService,
    },
  ],
  exports: [UserLookupService, RoleGuard, 'USER_SERVICE'],
})
export class AuthModule {}