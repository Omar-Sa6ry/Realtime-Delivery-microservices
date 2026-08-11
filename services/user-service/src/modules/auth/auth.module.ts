import { Module } from '@nestjs/common';
import { AuthService } from './auth.service';
import { AuthResolver } from './auth.resolver';
import { UserFactory } from './user.factory';
import { UserModule } from '../user/user.module';

@Module({
  imports: [UserModule],
  providers: [AuthService, AuthResolver, UserFactory],
  exports: [AuthService],
})
export class AuthModule {}
