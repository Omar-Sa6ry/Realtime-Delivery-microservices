import { UserAggregate } from '../aggregates/user.aggregate';

export interface IUserRepository {
  findById(id: string): Promise<UserAggregate | null>;
  findByEmail(email: string): Promise<UserAggregate | null>;
  findByResetToken(token: string): Promise<UserAggregate | null>;
  findUsers(page: number, limit: number): Promise<{ items: UserAggregate[]; total: number }>;
  countUsersThisMonthAndLastMonth(): Promise<{ totalUsers: number; usersThisMonth: number; percentageIncrease: number }>;
  save(user: UserAggregate): Promise<UserAggregate>;
  delete(id: string): Promise<boolean>;
}

export const IUSER_REPOSITORY = Symbol('IUserRepository');
