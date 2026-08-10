import { QueryHandler, IQueryHandler } from '@nestjs/cqrs';
import { Inject } from '@nestjs/common';
import { FindUsersQuery } from './find-users.query';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';

@QueryHandler(FindUsersQuery)
export class FindUsersHandler implements IQueryHandler<FindUsersQuery> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
  ) {}

  async execute(query: FindUsersQuery): Promise<any> {
    const result = await this.userRepo.findUsers(query.page, query.limit);
    return {
      items: result.items.map((user) => ({
        id: user.getId(),
        email: user.getEmail().getValue(),
        firstName: user.getFirstName(),
        lastName: user.getLastName(),
        role: user.getRole(),
        createdAt: user.getCreatedAt(),
      })),
      total: result.total,
    };
  }
}
