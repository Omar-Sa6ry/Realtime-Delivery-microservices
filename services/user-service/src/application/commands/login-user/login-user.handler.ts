import { CommandHandler, ICommandHandler } from '@nestjs/cqrs';
import { Inject, BadRequestException, UnauthorizedException } from '@nestjs/common';
import { LoginUserCommand } from './login-user.command';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';
import { IPASSWORD_HASHER } from '../../ports/password-hasher.port';
import type { IPasswordHasher } from '../../ports/password-hasher.port';
import { ITOKEN_PROVIDER } from '../../ports/token-provider.port';
import type { ITokenProvider } from '../../ports/token-provider.port';
import { ISESSION_REPOSITORY } from '../../../domain/repositories/session.repository.interface';
import type { ISessionRepository } from '../../../domain/repositories/session.repository.interface';
import { rolePermissionsMap } from '@delivery/common';

@CommandHandler(LoginUserCommand)
export class LoginUserHandler implements ICommandHandler<LoginUserCommand> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
    @Inject(IPASSWORD_HASHER) private readonly passwordHasher: IPasswordHasher,
    @Inject(ITOKEN_PROVIDER) private readonly tokenProvider: ITokenProvider,
    @Inject(ISESSION_REPOSITORY) private readonly sessionRepo: ISessionRepository,
  ) {}

  async execute(command: LoginUserCommand): Promise<any> {
    const user = await this.userRepo.findByEmail(command.email);
    if (!user) {
      throw new BadRequestException('Invalid credentials');
    }

    const isValidPassword = await this.passwordHasher.compare(
      command.password,
      user.getPasswordHash().getValue(),
    );
    if (!isValidPassword) {
      throw new UnauthorizedException('Invalid credentials');
    }

    const sessionId = crypto.randomUUID();
    const permissions = rolePermissionsMap[user.getRole()] || [];

    const tokens = await this.tokenProvider.generateTokens({
      userId: user.getId(),
      email: user.getEmail().getValue(),
      role: user.getRole(),
      permissions,
      sessionId,
    });

    await this.sessionRepo.createSession(
      user.getId(),
      sessionId,
      {
        userId: user.getId(),
        email: user.getEmail().getValue(),
        role: user.getRole(),
        sessionId,
        createdAt: new Date(),
      },
      tokens.expiresIn,
    );

    return {
      user: {
        id: user.getId(),
        email: user.getEmail().getValue(),
        firstName: user.getFirstName(),
        lastName: user.getLastName(),
        role: user.getRole(),
        createdAt: user.getCreatedAt(),
      },
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }
}
