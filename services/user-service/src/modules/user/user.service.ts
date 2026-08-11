import { Injectable, BadRequestException, UnauthorizedException, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, DataSource, MoreThanOrEqual, LessThanOrEqual } from 'typeorm';
import { User } from '../../common/database/entities/user.entity';
import { Address } from '../../common/database/entities/address.entity';
import { Outbox } from '../../common/database/entities/outbox.entity';
import { RedisService } from '@bts-soft/core';
import { BcryptPasswordHasher } from '../../common/security/bcrypt-password.hasher';
import { OutboxWorkerService } from '../../common/messaging/outbox-worker.service';
import { Role } from '@delivery/common';
import { IdGenerator } from '@bts-soft/core';

@Injectable()
export class UserService {
  constructor(
    @InjectRepository(User)
    private readonly userRepo: Repository<User>,
    @InjectRepository(Address)
    private readonly addressRepo: Repository<Address>,
    private readonly redis: RedisService,
    private readonly passwordHasher: BcryptPasswordHasher,
    private readonly dataSource: DataSource,
    private readonly outboxWorkerService: OutboxWorkerService,
  ) {}

  async findById(id: string): Promise<User | null> {
    const cached = await this.redis.hGet<string>(`user:${id}`, 'data');
    if (cached) {
      try {
        const orm = JSON.parse(cached);
        if (orm.createdAt) orm.createdAt = new Date(orm.createdAt);
        if (orm.updatedAt) orm.updatedAt = new Date(orm.updatedAt);
        if (orm.addresses) {
          orm.addresses.forEach((addr: any) => {
            if (addr.createdAt) addr.createdAt = new Date(addr.createdAt);
          });
        }
        return orm as User;
      } catch (err) {
        // Fall back to DB
      }
    }
    const user = await this.userRepo.findOne({ where: { id } });
    if (user) {
      await this.redis.hSet(`user:${id}`, 'data', JSON.stringify(user));
      await this.redis.expire(`user:${id}`, 3600);
      return user;
    }
    return null;
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
    firstName?: string,
    lastName?: string,
    phoneNumber?: string,
  ): Promise<User> {
    const user = await this.findById(userId);
    if (!user) {
      throw new BadRequestException('User not found');
    }

    if (firstName) user.firstName = firstName;
    if (lastName) user.lastName = lastName;
    if (phoneNumber !== undefined) {
      user.phoneNumber = phoneNumber ? phoneNumber.trim() : undefined;
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

    // Refresh Redis cache
    await this.redis.hSet(`user:${savedUser.id}`, 'data', JSON.stringify(savedUser));
    await this.redis.expire(`user:${savedUser.id}`, 3600);

    return savedUser;
  }

  async changePassword(userId: string, passwordOld: string, passwordNew: string): Promise<void> {
    const user = await this.findById(userId);
    if (!user) {
      throw new BadRequestException('User not found');
    }

    const isValid = await this.passwordHasher.compare(passwordOld, user.passwordHash);
    if (!isValid) {
      throw new UnauthorizedException('Invalid current password');
    }

    user.passwordHash = await this.passwordHasher.hash(passwordNew);
    
    const saved = await this.userRepo.save(user);
    await this.redis.hSet(`user:${saved.id}`, 'data', JSON.stringify(saved));
    await this.redis.expire(`user:${saved.id}`, 3600);
  }

  async promoteUserToAdmin(id: string): Promise<void> {
    const user = await this.findById(id);
    if (!user) {
      throw new NotFoundException('User not found');
    }

    user.role = Role.ADMIN;
    
    const saved = await this.userRepo.save(user);
    await this.redis.hSet(`user:${saved.id}`, 'data', JSON.stringify(saved));
    await this.redis.expire(`user:${saved.id}`, 3600);
  }

  async toggleUserActive(id: string, isActive: boolean): Promise<void> {
    const user = await this.findById(id);
    if (!user) {
      throw new NotFoundException('User not found');
    }

    user.isActive = isActive;
    
    const saved = await this.userRepo.save(user);
    await this.redis.hSet(`user:${saved.id}`, 'data', JSON.stringify(saved));
    await this.redis.expire(`user:${saved.id}`, 3600);
  }

  async deleteUser(id: string): Promise<boolean> {
    const user = await this.findById(id);
    if (!user) {
      throw new NotFoundException('User not found');
    }

    const result = await this.userRepo.delete(id);
    await this.redis.del(`user:${id}`);
    return (result.affected ?? 0) > 0;
  }

  async addAddress(userId: string, input: any): Promise<Address> {
    const user = await this.findById(userId);
    if (!user) {
      throw new NotFoundException('User not found');
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

    await this.addressRepo.save(address);

    // Refresh user cache
    const updatedUser = await this.userRepo.findOne({ where: { id: userId } });
    if (updatedUser) {
      await this.redis.hSet(`user:${userId}`, 'data', JSON.stringify(updatedUser));
      await this.redis.expire(`user:${userId}`, 3600);
    }

    return address;
  }

  async deleteAddress(userId: string, addressId: string): Promise<void> {
    const user = await this.findById(userId);
    if (!user) {
      throw new NotFoundException('User not found');
    }

    const hasAddress = user.addresses?.some(a => a.id === addressId);
    if (!hasAddress) {
      throw new BadRequestException('Address not found on this user profile');
    }

    await this.addressRepo.delete(addressId);

    // Refresh user cache
    const updatedUser = await this.userRepo.findOne({ where: { id: userId } });
    if (updatedUser) {
      await this.redis.hSet(`user:${userId}`, 'data', JSON.stringify(updatedUser));
      await this.redis.expire(`user:${userId}`, 3600);
    }
  }

  async setDefaultAddress(userId: string, addressId: string): Promise<void> {
    const user = await this.findById(userId);
    if (!user) {
      throw new NotFoundException('User not found');
    }

    const hasAddress = user.addresses?.some(a => a.id === addressId);
    if (!hasAddress) {
      throw new BadRequestException('Address not found on this user profile');
    }

    if (user.addresses) {
      user.addresses.forEach(addr => {
        addr.isDefault = addr.id === addressId;
      });
      await this.addressRepo.save(user.addresses);
    }

    // Refresh user cache
    const updatedUser = await this.userRepo.findOne({ where: { id: userId } });
    if (updatedUser) {
      await this.redis.hSet(`user:${userId}`, 'data', JSON.stringify(updatedUser));
      await this.redis.expire(`user:${userId}`, 3600);
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
