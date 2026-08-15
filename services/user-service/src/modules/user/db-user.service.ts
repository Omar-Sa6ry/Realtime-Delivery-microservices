import { Injectable, BadRequestException, UnauthorizedException, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, DataSource, MoreThanOrEqual, LessThanOrEqual } from 'typeorm';
import { User } from '../../common/database/entities/user.entity';
import { Address } from '../../common/database/entities/address.entity';
import { Outbox } from '../../common/database/entities/outbox.entity';
import { BcryptPasswordHasher } from '../../common/security/bcrypt-password.hasher';
import { OutboxWorkerService } from '../../common/messaging/outbox-worker.service';
import { Role } from '@delivery/common';
import { IdGenerator } from '@bts-soft/core';
import { I18nService } from 'nestjs-i18n';
import { UpdateProfileInput, ChangePasswordInput } from './dto/user.types';
import { MediaGrpcService } from '../media/grpc-media.service';

@Injectable()
export class DbUserService {
  constructor(
    @InjectRepository(User)
    private readonly userRepo: Repository<User>,
    @InjectRepository(Address)
    private readonly addressRepo: Repository<Address>,
    private readonly passwordHasher: BcryptPasswordHasher,
    private readonly dataSource: DataSource,
    private readonly outboxWorkerService: OutboxWorkerService,
    private readonly mediaGrpcService: MediaGrpcService,
    private readonly i18n: I18nService,
  ) {}

  async findById(id: string): Promise<User | null> {
    return this.userRepo.findOne({ where: { id } });
  }

  async findByEmail(email: string): Promise<User | null> {
    return this.userRepo.findOne({ where: { email: email.toLowerCase().trim() } });
  }

  async findUsers(page: number, limit: number): Promise<{ items: User[]; total: number }> {
    const [items, total] = await this.userRepo.findAndCount({
      where: { role: Role.USER },
      order: { createdAt: 'DESC' },
      skip: (page - 1) * limit,
      take: limit,
    });
    return { items, total };
  }

  async updateProfile(
    userId: string,
    input: UpdateProfileInput,
  ): Promise<User> {
    const user = await this.findById(userId);
    if (!user) {
      throw new BadRequestException(this.i18n.t('user.NOT_FOUND'));
    }

    const { firstName, lastName, imageUrl, avatarMediaId } = input;
    if (firstName) user.firstName = firstName;
    if (lastName) user.lastName = lastName;

    if (avatarMediaId) {
      try {
        const resolved = await this.mediaGrpcService.resolveMediaUrl({
          mediaId: avatarMediaId,
          requesterId: userId,
          versionType: 'original',
          expirySeconds: 3600,
        });
        user.imageUrl = resolved.url;
      } catch (err) {
        console.error('Failed to resolve avatar media URL, retrying once after delay:', err);
        // Retry after a short delay in case of race condition with media status transition
        await new Promise(resolve => setTimeout(resolve, 1000));
        try {
          const resolved = await this.mediaGrpcService.resolveMediaUrl({
            mediaId: avatarMediaId,
            requesterId: userId,
            versionType: 'original',
            expirySeconds: 3600,
          });
          user.imageUrl = resolved.url;
        } catch (retryErr) {
          console.error('Retry also failed for avatar URL resolution:', retryErr);
          // Store the mediaId so we can resolve later
          user.imageUrl = undefined;
        }
      }
    } else if (imageUrl) {
      if (!isHttpUrl(imageUrl)) {
        throw new BadRequestException(this.i18n.t('user.INVALID_IMAGE_URL'));
      }
      user.imageUrl = imageUrl;
    }

    const queryRunner = this.dataSource.createQueryRunner();
    await queryRunner.connect();
    await queryRunner.startTransaction();

    let savedUser: User;
    let outboxId: string;

    try {
      savedUser = await queryRunner.manager.save(user);

      // Create outbox event
      const outbox = new Outbox();
      outbox.id = crypto.randomUUID();
      outbox.aggregateType = 'User';
      outbox.aggregateId = savedUser.id;
      outbox.eventType = 'user.updated';
      outbox.payload = {
        userId: savedUser.id,
        email: savedUser.email,
        firstName: savedUser.firstName,
        lastName: savedUser.lastName,
        role: savedUser.role,
        imageUrl: savedUser.imageUrl || null,
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

    return savedUser;
  }

  async changePassword(userId: string, input: ChangePasswordInput): Promise<User> {
    const user = await this.findById(userId);
    if (!user) {
      throw new BadRequestException(this.i18n.t('user.NOT_FOUND'));
    }

    const isValid = await this.passwordHasher.compare(input.passwordOld, user.passwordHash);
    if (!isValid) {
      throw new UnauthorizedException(this.i18n.t('user.INVALID_PASSWORD'));
    }

    user.passwordHash = await this.passwordHasher.hash(input.passwordNew);
    return this.userRepo.save(user);
  }

  async promoteUserToAdmin(id: string): Promise<User> {
    const user = await this.findById(id);
    if (!user) {
      throw new NotFoundException(this.i18n.t('user.NOT_FOUND'));
    }

    user.role = Role.ADMIN;
    return this.userRepo.save(user);
  }

  async toggleUserActive(id: string, isActive: boolean): Promise<User> {
    const user = await this.findById(id);
    if (!user) {
      throw new NotFoundException(this.i18n.t('user.NOT_FOUND'));
    }

    user.isActive = isActive;
    return this.userRepo.save(user);
  }

  async deleteUser(id: string): Promise<boolean> {
    const user = await this.findById(id);
    if (!user) {
      throw new NotFoundException(this.i18n.t('user.NOT_FOUND'));
    }

    const result = await this.userRepo.delete(id);
    return (result.affected ?? 0) > 0;
  }

  async addAddress(userId: string, input: any): Promise<Address> {
    const user = await this.findById(userId);
    if (!user) {
      throw new NotFoundException(this.i18n.t('user.NOT_FOUND'));
    }

    const address = new Address();
    address.id = IdGenerator.generate('snowflake');
    address.userId = userId;
    address.title = input.title;
    address.street = input.street;
    address.city = input.city;
    address.state = input.state;
    address.postalCode = input.postalCode;
    address.latitude = input.latitude;
    address.longitude = input.longitude;
    address.isDefault = input.isDefault ?? false;

    if (address.isDefault) {
      // Set all other user addresses to isDefault = false
      if (user.addresses && user.addresses.length > 0) {
        user.addresses.forEach(addr => {
          addr.isDefault = false;
        });
        await this.addressRepo.save(user.addresses);
      }
    }

    return this.addressRepo.save(address);
  }

  async deleteAddress(userId: string, addressId: string): Promise<void> {
    const user = await this.findById(userId);
    if (!user) {
      throw new NotFoundException(this.i18n.t('user.NOT_FOUND'));
    }

    const hasAddress = user.addresses?.some(a => a.id === addressId);
    if (!hasAddress) {
      throw new BadRequestException(this.i18n.t('user.ADDRESS_NOT_FOUND'));
    }

    await this.addressRepo.delete(addressId);
  }

  async setDefaultAddress(userId: string, addressId: string): Promise<void> {
    const user = await this.findById(userId);
    if (!user) {
      throw new NotFoundException(this.i18n.t('user.NOT_FOUND'));
    }

    const hasAddress = user.addresses?.some(a => a.id === addressId);
    if (!hasAddress) {
      throw new BadRequestException(this.i18n.t('user.ADDRESS_NOT_FOUND'));
    }

    if (user.addresses) {
      user.addresses.forEach(addr => {
        addr.isDefault = addr.id === addressId;
      });
      await this.addressRepo.save(user.addresses);
    }
  }

  async countUsersThisMonthAndLastMonth(): Promise<{ totalUsers: number; usersThisMonth: number; percentageIncrease: number }> {
    const totalUsers = await this.userRepo.count({ where: { role: Role.USER } });
    const now = new Date();
    const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
    const startOfLastMonth = new Date(now.getFullYear(), now.getMonth() - 1, 1);
    const endOfLastMonth = new Date(now.getFullYear(), now.getMonth(), 0);

    const usersThisMonth = await this.userRepo.count({
      where: { role: Role.USER, createdAt: MoreThanOrEqual(startOfMonth) },
    });

    const usersLastMonth = await this.userRepo.count({
      where: {
        role: Role.USER,
        createdAt: MoreThanOrEqual(startOfLastMonth) && LessThanOrEqual(endOfLastMonth),
      },
    });

    const percentageIncrease = usersLastMonth
      ? Number((((usersThisMonth - usersLastMonth) / usersLastMonth) * 100).toFixed(2))
      : 100;

    return { totalUsers, usersThisMonth, percentageIncrease };
  }
}

function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
}
