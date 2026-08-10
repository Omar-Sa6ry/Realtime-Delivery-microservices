import { Resolver, Mutation, Args, Context } from '@nestjs/graphql';
import { CommandBus } from '@nestjs/cqrs';
import { 
  AuthPayloadType, 
  RegisterInput, 
  LoginInput, 
  ForgetPasswordInput, 
  ResetPasswordInput,
  RefreshTokenInput 
} from '../dtos/auth.types';
import { RegisterUserCommand } from '../../../application/commands/register-user/register-user.command';
import { LoginUserCommand } from '../../../application/commands/login-user/login-user.command';
import { ForgetPasswordCommand } from '../../../application/commands/forget-password/forget-password.command';
import { ResetPasswordCommand } from '../../../application/commands/reset-password/reset-password.command';
import { LogoutCommand } from '../../../application/commands/logout/logout.command';
import { RefreshTokenCommand } from '../../../application/commands/refresh-token/refresh-token.command';
import { UseGuards } from '@nestjs/common';
import { RoleGuard } from '@delivery/common';

@Resolver()
export class AuthResolver {
  constructor(private readonly commandBus: CommandBus) {}

  @Mutation(() => AuthPayloadType)
  async register(@Args('input') input: RegisterInput): Promise<AuthPayloadType> {
    return this.commandBus.execute(
      new RegisterUserCommand(
        input.email,
        input.password,
        input.firstName,
        input.lastName,
        input.phoneNumber,
      ),
    );
  }

  @Mutation(() => AuthPayloadType)
  async login(@Args('input') input: LoginInput): Promise<AuthPayloadType> {
    return this.commandBus.execute(
      new LoginUserCommand(input.email, input.password),
    );
  }

  @Mutation(() => Boolean)
  async forgetPassword(@Args('input') input: ForgetPasswordInput): Promise<boolean> {
    await this.commandBus.execute(new ForgetPasswordCommand(input.email));
    return true;
  }

  @Mutation(() => Boolean)
  async resetPassword(@Args('input') input: ResetPasswordInput): Promise<boolean> {
    await this.commandBus.execute(new ResetPasswordCommand(input.token, input.passwordNew));
    return true;
  }

  @Mutation(() => Boolean)
  @UseGuards(RoleGuard)
  async logout(@Context() ctx: any): Promise<boolean> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    const sessionId = ctx.req.user?.sessionId || ctx.req.headers['x-session-id'];
    await this.commandBus.execute(new LogoutCommand(userId, sessionId));
    return true;
  }

  @Mutation(() => AuthPayloadType)
  async refreshToken(@Args('input') input: RefreshTokenInput): Promise<AuthPayloadType> {
    return this.commandBus.execute(new RefreshTokenCommand(input.refreshToken));
  }
}
