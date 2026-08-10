import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, MoreThanOrEqual, LessThanOrEqual } from 'typeorm';
import { UserOrmEntity } from '../entities/user.orm-entity';
import { UserAggregate } from '../../../domain/aggregates/user.aggregate';
import { IUserRepository } from '../../../domain/repositories/user.repository.interface';
import { UserMapper } from '../mappers/user.mapper';
import { Role } from '@delivery/common';

@Injectable()
export class TypeOrmUserRepository implements IUserRepository {
  constructor(
    @InjectRepository(UserOrmEntity)
    private readonly userRepo: Repository<UserOrmEntity>,
  ) {}

  async findById(id: string): Promise<UserAggregate | null> {
    const orm = await this.userRepo.findOne({ where: { id } });
    return orm ? UserMapper.toDomain(orm) : null;
  }

  async findByEmail(email: string): Promise<UserAggregate | null> {
    const orm = await this.userRepo.findOne({ where: { email: email.toLowerCase().trim() } });
    return orm ? UserMapper.toDomain(orm) : null;
  }

  async findByResetToken(token: string): Promise<UserAggregate | null> {
    const orm = await this.userRepo.findOne({ where: { resetToken: token } });
    return orm ? UserMapper.toDomain(orm) : null;
  }

  async findUsers(page: number, limit: number): Promise<{ items: UserAggregate[]; total: number }> {
    const [items, total] = await this.userRepo.findAndCount({
      where: { role: Role.USER },
      order: { createdAt: 'DESC' },
      skip: (page - 1) * limit,
      take: limit,
    });
    return {
      items: items.map((orm) => UserMapper.toDomain(orm)),
      total,
    };
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

  async save(user: UserAggregate): Promise<UserAggregate> {
    const ormEntity = UserMapper.toOrm(user);
    const saved = await this.userRepo.save(ormEntity);
    return UserMapper.toDomain(saved);
  }

  async delete(id: string): Promise<boolean> {
    const result = await this.userRepo.delete(id);
    return (result.affected ?? 0) > 0;
  }
}
