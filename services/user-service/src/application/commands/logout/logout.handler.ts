import { CommandHandler, ICommandHandler } from '@nestjs/cqrs';
import { Inject } from '@nestjs/common';
import { LogoutCommand } from './logout.command';
import { ISESSION_REPOSITORY } from '../../../domain/repositories/session.repository.interface';
import type { ISessionRepository } from '../../../domain/repositories/session.repository.interface';

@CommandHandler(LogoutCommand)
export class LogoutHandler implements ICommandHandler<LogoutCommand> {
  constructor(
    @Inject(ISESSION_REPOSITORY) private readonly sessionRepo: ISessionRepository,
  ) {}

  async execute(command: LogoutCommand): Promise<void> {
    await this.sessionRepo.revokeSession(command.userId, command.sessionId);
  }
}
