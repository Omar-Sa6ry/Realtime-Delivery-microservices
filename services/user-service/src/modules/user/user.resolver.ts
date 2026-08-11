import { Resolver, Query, Mutation, Args, Context } from '@nestjs/graphql';
import { 
  UserType, 
  UserResponse, 
  PaginatedUsers, 
  PaginatedUsersResponse, 
  ChangePasswordInput, 
  UpdateProfileInput,
  AddAddressInput,
  AddressResponse,
  AddressListResponse
} from './dto/user.types';
import { BooleanResponse } from '../../common/graphql/general-response.type';
import { UserService } from './user.service';
import { Auth, Permission } from '@delivery/common';

@Resolver(() => UserType)
export class UserResolver {
  constructor(private readonly userService: UserService) {}

  @Query(() => UserResponse)
  async user(@Args('id') id: string): Promise<any> {
    return this.userService.findById(id);
  }

  @Query(() => UserResponse)
  @Auth()
  async myProfile(@Context() ctx: any): Promise<any> {
    const userId = ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    return this.userService.findById(userId);
  }

  @Mutation(() => BooleanResponse)
  @Auth()
  async changePassword(
    @Context() ctx: any,
    @Args('input') input: ChangePasswordInput,
  ): Promise<boolean> {
    const userId = ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    await this.userService.changePassword(userId, input.passwordOld, input.passwordNew);
    return true;
  }

  @Mutation(() => UserResponse)
  @Auth()
  async updateProfile(
    @Context() ctx: any,
    @Args('input') input: UpdateProfileInput,
  ): Promise<any> {
    const userId = ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    return this.userService.updateProfile(
      userId,
      input.firstName,
      input.lastName,
      input.phoneNumber,
    );
  }

  @Query(() => PaginatedUsersResponse)
  @Auth()
  async users(
    @Args('page', { defaultValue: 1 }) page: number,
    @Args('limit', { defaultValue: 10 }) limit: number,
  ): Promise<any> {
    return this.userService.findUsers(page, limit);
  }

  @Mutation(() => BooleanResponse)
  @Auth([Permission.EDIT_USER_ROLE])
  async promoteUserToAdmin(@Args('id') id: string): Promise<boolean> {
    await this.userService.promoteUserToAdmin(id);
    return true;
  }

  @Mutation(() => BooleanResponse)
  @Auth([Permission.EDIT_USER_ROLE])
  async toggleUserActive(
    @Args('id') id: string,
    @Args('isActive') isActive: boolean,
  ): Promise<boolean> {
    await this.userService.toggleUserActive(id, isActive);
    return true;
  }

  @Mutation(() => BooleanResponse)
  @Auth([Permission.DELETE_USER])
  async deleteUser(@Args('id') id: string): Promise<boolean> {
    await this.userService.deleteUser(id);
    return true;
  }

  @Mutation(() => AddressResponse)
  @Auth()
  async addAddress(
    @Context() ctx: any,
    @Args('input') input: AddAddressInput,
  ): Promise<any> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    return this.userService.addAddress(userId, input);
  }

  @Mutation(() => BooleanResponse)
  @Auth()
  async deleteAddress(
    @Context() ctx: any,
    @Args('addressId') addressId: string,
  ): Promise<boolean> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    await this.userService.deleteAddress(userId, addressId);
    return true;
  }

  @Query(() => AddressListResponse)
  @Auth()
  async myAddresses(@Context() ctx: any): Promise<any> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    const user = await this.userService.findById(userId);
    return {
      success: true,
      statusCode: 200,
      message: 'Addresses retrieved successfully',
      data: user?.addresses || [],
    };
  }

  @Mutation(() => BooleanResponse)
  @Auth()
  async setDefaultAddress(
    @Context() ctx: any,
    @Args('addressId') addressId: string,
  ): Promise<boolean> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    await this.userService.setDefaultAddress(userId, addressId);
    return true;
  }
}
