import { CommandHandler, ICommandHandler } from '@nestjs/cqrs';
import { Inject, BadRequestException } from '@nestjs/common';
import { UpdateProfileCommand } from './update-profile.command';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';
import { PhoneNumber } from '../../../domain/value-objects/phone-number.vo';
import { DataSource } from 'typeorm';
import { OutboxOrmEntity } from '../../../infrastructure/database/entities/outbox.orm-entity';
import { OutboxWorkerService } from '../../../infrastructure/messaging/outbox-worker.service';
import { UserMapper } from '../../../infrastructure/database/mappers/user.mapper';
import { UserAggregate } from '../../../domain/aggregates/user.aggregate';

@CommandHandler(UpdateProfileCommand)
export class UpdateProfileHandler implements ICommandHandler<UpdateProfileCommand> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
    private readonly dataSource: DataSource,
    private readonly outboxWorkerService: OutboxWorkerService,
  ) {}

  async execute(command: UpdateProfileCommand): Promise<any> {
    const user = await this.userRepo.findById(command.userId);
    if (!user) {
      throw new BadRequestException('User not found');
    }

    const phoneNumber = command.phoneNumber ? new PhoneNumber(command.phoneNumber) : undefined;
    user.updateProfile(command.firstName, command.lastName, phoneNumber);
    
    const queryRunner = this.dataSource.createQueryRunner();
    await queryRunner.connect();
    await queryRunner.startTransaction();

    let savedUser: UserAggregate;
    let outboxId: string;

    try {
      const ormUser = UserMapper.toOrm(user);
      const savedOrmUser = await queryRunner.manager.save(ormUser);
      savedUser = UserMapper.toDomain(savedOrmUser);

      // Create outbox event
      const outbox = new OutboxOrmEntity();
      outbox.id = crypto.randomUUID();
      outbox.aggregateType = 'User';
      outbox.aggregateId = savedUser.getId();
      outbox.eventType = 'user.updated';
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

    // Immediate dispatching via BullMQ
    await this.outboxWorkerService.enqueueEvent(outboxId);

    return {
      id: savedUser.getId(),
      email: savedUser.getEmail().getValue(),
      firstName: savedUser.getFirstName(),
      lastName: savedUser.getLastName(),
      role: savedUser.getRole(),
      createdAt: savedUser.getCreatedAt(),
    };
  }
}
