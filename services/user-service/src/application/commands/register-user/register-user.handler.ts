import { CommandHandler, ICommandHandler } from '@nestjs/cqrs';
import { Inject, BadRequestException } from '@nestjs/common';
import { RegisterUserCommand } from './register-user.command';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';
import { IPASSWORD_HASHER } from '../../ports/password-hasher.port';
import type { IPasswordHasher } from '../../ports/password-hasher.port';
import { ITOKEN_PROVIDER } from '../../ports/token-provider.port';
import type { ITokenProvider } from '../../ports/token-provider.port';
import { ISESSION_REPOSITORY } from '../../../domain/repositories/session.repository.interface';
import type { ISessionRepository } from '../../../domain/repositories/session.repository.interface';
import { UserAggregate } from '../../../domain/aggregates/user.aggregate';
import { Email } from '../../../domain/value-objects/email.vo';
import { PasswordHash } from '../../../domain/value-objects/password-hash.vo';
import { PhoneNumber } from '../../../domain/value-objects/phone-number.vo';
import { rolePermissionsMap } from '@delivery/common';
import { DataSource } from 'typeorm';
import { OutboxOrmEntity } from '../../../infrastructure/database/entities/outbox.orm-entity';
import { OutboxWorkerService } from '../../../infrastructure/messaging/outbox-worker.service';
import { UserMapper } from '../../../infrastructure/database/mappers/user.mapper';

@CommandHandler(RegisterUserCommand)
export class RegisterUserHandler implements ICommandHandler<RegisterUserCommand> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
    @Inject(IPASSWORD_HASHER) private readonly passwordHasher: IPasswordHasher,
    @Inject(ITOKEN_PROVIDER) private readonly tokenProvider: ITokenProvider,
    @Inject(ISESSION_REPOSITORY) private readonly sessionRepo: ISessionRepository,
    private readonly dataSource: DataSource,
    private readonly outboxWorkerService: OutboxWorkerService,
  ) {}

  async execute(command: RegisterUserCommand): Promise<any> {
    const email = new Email(command.email);
    const existing = await this.userRepo.findByEmail(email.getValue());
    if (existing) {
      throw new BadRequestException('Email already exists');
    }

    const hashedPassword = await this.passwordHasher.hash(command.password);
    const passwordHash = new PasswordHash(hashedPassword);
    const phoneNumber = command.phoneNumber ? new PhoneNumber(command.phoneNumber) : undefined;

    const user = new UserAggregate({
      email,
      passwordHash,
      firstName: command.firstName,
      lastName: command.lastName,
      phoneNumber,
    });

    const queryRunner = this.dataSource.createQueryRunner();
    await queryRunner.connect();
    await queryRunner.startTransaction();

    let savedUser: UserAggregate;
    let outboxId: string;

    try {
      const ormUser = UserMapper.toOrm(user);
      const savedOrmUser = await queryRunner.manager.save(ormUser);
      savedUser = UserMapper.toDomain(savedOrmUser);

      // Create outbox record
      const outbox = new OutboxOrmEntity();
      outbox.id = crypto.randomUUID();
      outbox.aggregateType = 'User';
      outbox.aggregateId = savedUser.getId();
      outbox.eventType = 'user.registered';
      outbox.payload = {
        userId: savedUser.getId(),
        email: savedUser.getEmail().getValue(),
        firstName: savedUser.getFirstName(),
        lastName: savedUser.getLastName(),
        role: savedUser.getRole(),
      };
      outbox.processed = false;

      const savedOutbox = await queryRunner.manager.save(outbox);
      outboxId = savedOutbox.id;

      await queryRunner.commitTransaction();
    } catch (err) {
      await queryRunner.rollbackTransaction();
      throw err;
    } finally {
      await queryRunner.release();
    }

    // Enqueue event to Redis BullMQ
    await this.outboxWorkerService.enqueueEvent(outboxId);

    const sessionId = crypto.randomUUID();
    const permissions = rolePermissionsMap[savedUser.getRole()] || [];

    const tokens = await this.tokenProvider.generateTokens({
      userId: savedUser.getId(),
      email: savedUser.getEmail().getValue(),
      role: savedUser.getRole(),
      permissions,
      sessionId,
    });

    await this.sessionRepo.createSession(
      savedUser.getId(),
      sessionId,
      {
        userId: savedUser.getId(),
        email: savedUser.getEmail().getValue(),
        role: savedUser.getRole(),
        sessionId,
        createdAt: new Date(),
      },
      tokens.expiresIn,
    );

    return {
      user: {
        id: savedUser.getId(),
        email: savedUser.getEmail().getValue(),
        firstName: savedUser.getFirstName(),
        lastName: savedUser.getLastName(),
        role: savedUser.getRole(),
        createdAt: savedUser.getCreatedAt(),
      },
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }
}
