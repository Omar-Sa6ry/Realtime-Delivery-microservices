import { Resolver, Mutation, Args, Context } from '@nestjs/graphql';
import { UseGuards } from '@nestjs/common';
import { 
  AuthPayloadType, 
  AuthResponse,
  RegisterInput, 
  LoginInput, 
  ForgetPasswordInput, 
  ResetPasswordInput,
  RefreshTokenInput
} from './dto/auth.types';
import { BooleanResponse } from '../../common/graphql/general-response.type';
import { AuthService } from './auth.service';
import { RoleGuard, RedisRateLimit, RateLimiterAlgorithm } from '@delivery/common';

@Resolver()
export class AuthResolver {
  constructor(private readonly authService: AuthService) {}

  @Mutation(() => AuthResponse)
  @RedisRateLimit({
    algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER,
    limit: 3,
    windowMs: 60000,
  })
  async register(@Args('input') input: RegisterInput): Promise<AuthPayloadType> {
    return this.authService.register(
      input.email,
      input.password,
      input.firstName,
      input.lastName,
      input.phoneNumber,
    );
  }

  @Mutation(() => AuthResponse)
  @RedisRateLimit({
    algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER,
    limit: 5,
    windowMs: 60000,
  })
  async login(@Args('input') input: LoginInput): Promise<AuthPayloadType> {
    return this.authService.login(input.email, input.password);
  }

  @Mutation(() => BooleanResponse)
  @RedisRateLimit({
    algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER,
    limit: 3,
    windowMs: 60000,
  })
  async forgetPassword(@Args('input') input: ForgetPasswordInput): Promise<boolean> {
    await this.authService.forgetPassword(input.email);
    return true;
  }

  @Mutation(() => BooleanResponse)
  @RedisRateLimit({
    algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER,
    limit: 3,
    windowMs: 60000,
  })
  async resetPassword(@Args('input') input: ResetPasswordInput): Promise<boolean> {
    await this.authService.resetPassword(input.token, input.passwordNew);
    return true;
  }

  @Mutation(() => BooleanResponse)
  @UseGuards(RoleGuard)
  async logout(@Context() ctx: any): Promise<boolean> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    const sessionId = ctx.req.user?.sessionId || ctx.req.headers['x-session-id'];
    await this.authService.logout(userId, sessionId);
    return true;
  }

  @Mutation(() => AuthResponse)
  async refreshToken(@Args('input') input: RefreshTokenInput): Promise<AuthPayloadType> {
    return this.authService.refreshToken(input.refreshToken);
  }
}
