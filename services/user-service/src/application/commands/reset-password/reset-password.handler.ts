import { CommandHandler, ICommandHandler } from '@nestjs/cqrs';
import { Inject, BadRequestException } from '@nestjs/common';
import { ResetPasswordCommand } from './reset-password.command';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';
import { IPASSWORD_HASHER } from '../../ports/password-hasher.port';
import type { IPasswordHasher } from '../../ports/password-hasher.port';
import { PasswordHash } from '../../../domain/value-objects/password-hash.vo';

@CommandHandler(ResetPasswordCommand)
export class ResetPasswordHandler implements ICommandHandler<ResetPasswordCommand> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
    @Inject(IPASSWORD_HASHER) private readonly passwordHasher: IPasswordHasher,
  ) {}

  async execute(command: ResetPasswordCommand): Promise<void> {
    const user = await this.userRepo.findByResetToken(command.token);
    if (!user || !user.isResetTokenValid(command.token)) {
      throw new BadRequestException('Invalid or expired reset token');
    }

    const hashedPassword = await this.passwordHasher.hash(command.passwordNew);
    user.changePassword(new PasswordHash(hashedPassword));
    user.clearResetToken();

    await this.userRepo.save(user);
  }
}
