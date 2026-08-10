import { Resolver, Query, Mutation, Args, Context } from '@nestjs/graphql';
import { QueryBus, CommandBus } from '@nestjs/cqrs';
import { UserType, PaginatedUsers, ChangePasswordInput, UpdateProfileInput } from '../dtos/auth.types';
import { GetUserByIdQuery } from '../../../application/queries/get-user-by-id/get-user-by-id.query';
import { FindUsersQuery } from '../../../application/queries/find-users/find-users.query';
import { ChangePasswordCommand } from '../../../application/commands/change-password/change-password.command';
import { UpdateProfileCommand } from '../../../application/commands/update-profile/update-profile.command';
import { UseGuards } from '@nestjs/common';
import { RoleGuard } from '@delivery/common';

@Resolver(() => UserType)
export class UserResolver {
  constructor(
    private readonly queryBus: QueryBus,
    private readonly commandBus: CommandBus,
  ) {}

  @Query(() => UserType)
  async user(@Args('id') id: string): Promise<UserType> {
    return this.queryBus.execute(new GetUserByIdQuery(id));
  }

  @Query(() => UserType)
  @UseGuards(RoleGuard)
  async myProfile(@Context() ctx: any): Promise<UserType> {
    const userId = ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    return this.queryBus.execute(new GetUserByIdQuery(userId));
  }

  @Mutation(() => Boolean)
  @UseGuards(RoleGuard)
  async changePassword(
    @Context() ctx: any,
    @Args('input') input: ChangePasswordInput,
  ): Promise<boolean> {
    const userId = ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    await this.commandBus.execute(new ChangePasswordCommand(userId, input.passwordOld, input.passwordNew));
    return true;
  }

  @Mutation(() => UserType)
  @UseGuards(RoleGuard)
  async updateProfile(
    @Context() ctx: any,
    @Args('input') input: UpdateProfileInput,
  ): Promise<UserType> {
    const userId = ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    return this.commandBus.execute(
      new UpdateProfileCommand(
        userId,
        input.firstName,
        input.lastName,
        input.phoneNumber,
      ),
    );
  }

  @Query(() => PaginatedUsers)
  @UseGuards(RoleGuard)
  async users(
    @Args('page', { defaultValue: 1 }) page: number,
    @Args('limit', { defaultValue: 10 }) limit: number,
  ): Promise<PaginatedUsers> {
    return this.queryBus.execute(new FindUsersQuery(page, limit));
  }
}
