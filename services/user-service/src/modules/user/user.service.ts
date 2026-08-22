import { Injectable } from '@nestjs/common';
import { RedisService } from '@bts-soft/core';
import { DbUserService } from './db-user.service';
import { User } from '../../common/database/entities/user.entity';
import { Address } from '../../common/database/entities/address.entity';
import { UpdateProfileInput, ChangePasswordInput } from './dto/user.types';

@Injectable()
export class UserService {
  constructor(
    private readonly dbUserService: DbUserService,
    private readonly redis: RedisService,
  ) {}

  private async refreshCache(user: User): Promise<void> {
    await this.redis.hSet(`user:${user.id}`, 'data', JSON.stringify(user));
    await this.redis.expire(`user:${user.id}`, 3600);
  }

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
    const user = await this.dbUserService.findById(id);
    if (user) {
      await this.refreshCache(user);
      return user;
    }
    return null;
  }

  async findByEmail(email: string): Promise<User | null> {
    return this.dbUserService.findByEmail(email);
  }

  async updateProfile(
    userId: string,
    input: UpdateProfileInput,
  ): Promise<User> {
    const updatedUser = await this.dbUserService.updateProfile(userId, input);
    await this.refreshCache(updatedUser);
    return updatedUser;
  }

  async changePassword(userId: string, input: ChangePasswordInput): Promise<void> {
    const updatedUser = await this.dbUserService.changePassword(userId, input);
    await this.refreshCache(updatedUser);
  }

  async promoteUserToAdmin(id: string): Promise<void> {
    const updatedUser = await this.dbUserService.promoteUserToAdmin(id);
    await this.refreshCache(updatedUser);
  }

  async toggleUserActive(id: string, isActive: boolean): Promise<void> {
    const updatedUser = await this.dbUserService.toggleUserActive(id, isActive);
    await this.refreshCache(updatedUser);
  }

  async deleteUser(id: string): Promise<boolean> {
    const deleted = await this.dbUserService.deleteUser(id);
    if (deleted) {
      await this.redis.del(`user:${id}`);
    }
    return deleted;
  }

  async addAddress(userId: string, input: any): Promise<Address> {
    const address = await this.dbUserService.addAddress(userId, input);
    
    // Refresh user cache
    const updatedUser = await this.dbUserService.findById(userId);
    if (updatedUser) {
      await this.refreshCache(updatedUser);
    }

    return address;
  }

  async deleteAddress(userId: string, addressId: string): Promise<void> {
    await this.dbUserService.deleteAddress(userId, addressId);
    
    // Refresh user cache
    const updatedUser = await this.dbUserService.findById(userId);
    if (updatedUser) {
      await this.refreshCache(updatedUser);
    }
  }

  async setDefaultAddress(userId: string, addressId: string): Promise<void> {
    await this.dbUserService.setDefaultAddress(userId, addressId);
    
    // Refresh user cache
    const updatedUser = await this.dbUserService.findById(userId);
    if (updatedUser) {
      await this.refreshCache(updatedUser);
    }
  }

  async countUsersThisMonthAndLastMonth(): Promise<{ totalUsers: number; usersThisMonth: number; percentageIncrease: number }> {
    return this.dbUserService.countUsersThisMonthAndLastMonth();
  }
}
