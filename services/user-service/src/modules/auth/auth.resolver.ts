import { Resolver, Mutation, Args, Context } from '@nestjs/graphql';
import { UseGuards } from '@nestjs/common';
import { I18nService } from 'nestjs-i18n';
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
  constructor(
    private readonly authService: AuthService,
    private readonly i18n: I18nService,
  ) {}

  @Mutation(() => AuthResponse)
  @RedisRateLimit({
    algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER,
    limit: 3,
    windowMs: 60000,
  })
  async register(@Args('input') input: RegisterInput): Promise<AuthResponse> {
    const data = await this.authService.register(input);
    return {
      success: true,
      statusCode: 201,
      message: await this.i18n.t('user.REGISTER_SUCCESS'),
      data,
    } as AuthResponse;
  }

  @Mutation(() => AuthResponse)
  @RedisRateLimit({
    algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER,
    limit: 5,
    windowMs: 60000,
  })
  async login(@Args('input') input: LoginInput): Promise<AuthResponse> {
    const data = await this.authService.login(input);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.LOGIN_SUCCESS'),
      data,
    } as AuthResponse;
  }

  @Mutation(() => BooleanResponse)
  @RedisRateLimit({
    algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER,
    limit: 3,
    windowMs: 60000,
  })
  async forgetPassword(@Args('input') input: ForgetPasswordInput): Promise<BooleanResponse> {
    await this.authService.forgetPassword(input);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.FORGET_PASSWORD_SUCCESS'),
      data: true,
    } as BooleanResponse;
  }

  @Mutation(() => BooleanResponse)
  @RedisRateLimit({
    algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER,
    limit: 3,
    windowMs: 60000,
  })
  async resetPassword(@Args('input') input: ResetPasswordInput): Promise<BooleanResponse> {
    await this.authService.resetPassword(input);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.RESET_PASSWORD_SUCCESS'),
      data: true,
    } as BooleanResponse;
  }

  @Mutation(() => BooleanResponse)
  @UseGuards(RoleGuard)
  async logout(@Context() ctx: any): Promise<BooleanResponse> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    const sessionId = ctx.req.user?.sessionId || ctx.req.headers['x-session-id'];
    await this.authService.logout(userId, sessionId);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.LOGOUT_SUCCESS'),
      data: true,
    } as BooleanResponse;
  }

  @Mutation(() => AuthResponse)
  async refreshToken(@Args('input') input: RefreshTokenInput): Promise<AuthResponse> {
    const data = await this.authService.refreshToken(input);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.REFRESH_TOKEN_SUCCESS'),
      data,
    } as AuthResponse;
  }
}
