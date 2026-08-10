import { CommandHandler, ICommandHandler } from '@nestjs/cqrs';
import { Inject } from '@nestjs/common';
import { ForgetPasswordCommand } from './forget-password.command';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';
import { NotificationService, ChannelType } from '@bts-soft/core';

@CommandHandler(ForgetPasswordCommand)
export class ForgetPasswordHandler implements ICommandHandler<ForgetPasswordCommand> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
    private readonly notificationService: NotificationService,
  ) {}

  async execute(command: ForgetPasswordCommand): Promise<void> {
    const user = await this.userRepo.findByEmail(command.email);
    if (!user) {
      // Return success silently for security (avoid user enumeration)
      return;
    }

    const token = Math.floor(100000 + Math.random() * 900000).toString(); // 6-digit code
    user.generateResetToken(token, 15); // Expiry in 15 mins
    await this.userRepo.save(user);

    // Send email using @bts-soft/notifications from @bts-soft/core
    await this.notificationService.send(ChannelType.EMAIL, {
      recipientId: user.getEmail().getValue(),
      subject: 'Password Reset Code',
      body: 'Hi {{name}}, your password reset code is {{token}}.',
      context: { name: user.getFirstName(), token },
    });
  }
}
