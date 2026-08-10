import { CommandHandler, ICommandHandler } from '@nestjs/cqrs';
import { Inject, BadRequestException, UnauthorizedException } from '@nestjs/common';
import { ChangePasswordCommand } from './change-password.command';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';
import { IPASSWORD_HASHER } from '../../ports/password-hasher.port';
import type { IPasswordHasher } from '../../ports/password-hasher.port';
import { PasswordHash } from '../../../domain/value-objects/password-hash.vo';

@CommandHandler(ChangePasswordCommand)
export class ChangePasswordHandler implements ICommandHandler<ChangePasswordCommand> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
    @Inject(IPASSWORD_HASHER) private readonly passwordHasher: IPasswordHasher,
  ) {}

  async execute(command: ChangePasswordCommand): Promise<void> {
    const user = await this.userRepo.findById(command.userId);
    if (!user) {
      throw new BadRequestException('User not found');
    }

    const isValid = await this.passwordHasher.compare(
      command.passwordOld,
      user.getPasswordHash().getValue(),
    );
    if (!isValid) {
      throw new UnauthorizedException('Invalid current password');
    }

    const hashedPassword = await this.passwordHasher.hash(command.passwordNew);
    user.changePassword(new PasswordHash(hashedPassword));

    await this.userRepo.save(user);
  }
}
