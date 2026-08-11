import { Module } from '@nestjs/common';
import { AuthService } from './auth.service';
import { AuthResolver } from './auth.resolver';
import { UserFactory } from './user.factory';

@Module({
  providers: [AuthService, AuthResolver, UserFactory],
  exports: [AuthService],
})
export class AuthModule {}
