import { IQueryHandler, QueryHandler } from '@nestjs/cqrs';
import { Inject, NotFoundException } from '@nestjs/common';
import { GetUserByIdQuery } from './get-user-by-id.query';
import { IUSER_REPOSITORY } from '../../../domain/repositories/user.repository.interface';
import type { IUserRepository } from '../../../domain/repositories/user.repository.interface';

@QueryHandler(GetUserByIdQuery)
export class GetUserByIdHandler implements IQueryHandler<GetUserByIdQuery> {
  constructor(
    @Inject(IUSER_REPOSITORY) private readonly userRepo: IUserRepository,
  ) {}

  async execute(query: GetUserByIdQuery): Promise<any> {
    const user = await this.userRepo.findById(query.id);
    if (!user) {
      throw new NotFoundException('User not found');
    }

    return {
      id: user.getId(),
      email: user.getEmail().getValue(),
      firstName: user.getFirstName(),
      lastName: user.getLastName(),
      role: user.getRole(),
      phoneNumber: user.getPhoneNumber()?.getValue(),
      isActive: user.getIsActive(),
      isVerified: user.getIsVerified(),
      createdAt: user.getCreatedAt(),
    };
  }
}
