import { CommandHandler, ICommandHandler } from '@nestjs/cqrs';
import { Inject, UnauthorizedException } from '@nestjs/common';
import { RefreshTokenCommand } from './refresh-token.command';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';
import { ITOKEN_PROVIDER } from '../../ports/token-provider.port';
import type { ITokenProvider } from '../../ports/token-provider.port';
import { ISESSION_REPOSITORY } from '../../../domain/repositories/session.repository.interface';
import type { ISessionRepository } from '../../../domain/repositories/session.repository.interface';
import { rolePermissionsMap } from '@delivery/common';

@CommandHandler(RefreshTokenCommand)
export class RefreshTokenHandler implements ICommandHandler<RefreshTokenCommand> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
    @Inject(ITOKEN_PROVIDER) private readonly tokenProvider: ITokenProvider,
    @Inject(ISESSION_REPOSITORY) private readonly sessionRepo: ISessionRepository,
  ) {}

  async execute(command: RefreshTokenCommand): Promise<any> {
    const payload = await this.tokenProvider.verifyRefreshToken(command.refreshToken);
    if (!payload || !payload.sessionId) {
      throw new UnauthorizedException('Invalid or expired refresh token');
    }

    const session = await this.sessionRepo.getSession(payload.userId, payload.sessionId);
    if (!session) {
      throw new UnauthorizedException('Session expired or logged out');
    }

    const user = await this.userRepo.findById(payload.userId);
    if (!user) {
      throw new UnauthorizedException('User not found');
    }

    const permissions = rolePermissionsMap[user.getRole()] || [];

    const tokens = await this.tokenProvider.generateTokens({
      userId: user.getId(),
      email: user.getEmail().getValue(),
      role: user.getRole(),
      permissions,
      sessionId: payload.sessionId,
    });

    // Refresh the session in Redis with same creation timestamp and updated expiry
    await this.sessionRepo.createSession(
      user.getId(),
      payload.sessionId,
      {
        userId: user.getId(),
        email: user.getEmail().getValue(),
        role: user.getRole(),
        sessionId: payload.sessionId,
        createdAt: session.createdAt,
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
