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
  AddressListResponse,
} from './dto/user.types';
import { BooleanResponse } from '../../common/graphql/general-response.type';
import { UserService } from './user.service';
import { Auth, Permission } from '@delivery/common';
import { I18nService } from 'nestjs-i18n';

@Resolver(() => UserType)
export class UserResolver {
  constructor(
    private readonly userService: UserService,
    private readonly i18n: I18nService,
  ) {}

  @Query(() => UserResponse)
  @Auth([Permission.VIEW_USER])
  async user(@Args('id') id: string): Promise<UserResponse> {
    const user = await this.userService.findById(id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.USER_RETRIEVED'),
      data: user || undefined,
    } as UserResponse;
  }

  @Query(() => UserResponse)
  @Auth()
  async myProfile(@Context() ctx: any): Promise<UserResponse> {
    const userId =
      ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    const user = await this.userService.findById(userId);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.PROFILE_RETRIEVED'),
      data: user || undefined,
    } as UserResponse;
  }

  @Mutation(() => BooleanResponse)
  @Auth()
  async changePassword(
    @Context() ctx: any,
    @Args('input') input: ChangePasswordInput,
  ): Promise<BooleanResponse> {
    const userId =
      ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    await this.userService.changePassword(userId, input);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.PASSWORD_CHANGED'),
      data: true,
    } as BooleanResponse;
  }

  @Mutation(() => UserResponse)
  @Auth()
  async updateProfile(
    @Context() ctx: any,
    @Args('input') input: UpdateProfileInput,
  ): Promise<UserResponse> {
    const userId =
      ctx.req.user?.id || ctx.req.user?.userId || ctx.req.headers['x-user-id'];
    const user = await this.userService.updateProfile(userId, input);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.PROFILE_UPDATED'),
      data: user,
    } as UserResponse;
  }

  @Query(() => PaginatedUsersResponse)
  @Auth([Permission.VIEW_USER])
  @Auth()
  async users(
    @Args('page', { defaultValue: 1 }) page: number,
    @Args('limit', { defaultValue: 10 }) limit: number,
  ): Promise<PaginatedUsersResponse> {
    const result = await this.userService.findUsers(page, limit);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.USERS_RETRIEVED'),
      data: result,
    } as PaginatedUsersResponse;
  }

  @Mutation(() => BooleanResponse)
  @Auth([Permission.EDIT_USER_ROLE])
  async promoteUserToAdmin(@Args('id') id: string): Promise<BooleanResponse> {
    await this.userService.promoteUserToAdmin(id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.USER_PROMOTED'),
      data: true,
    } as BooleanResponse;
  }

  @Mutation(() => BooleanResponse)
  @Auth([Permission.EDIT_USER_ROLE])
  async toggleUserActive(
    @Args('id') id: string,
    @Args('isActive') isActive: boolean,
  ): Promise<BooleanResponse> {
    await this.userService.toggleUserActive(id, isActive);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.USER_STATUS_UPDATED'),
      data: true,
    } as BooleanResponse;
  }

  @Mutation(() => BooleanResponse)
  @Auth([Permission.DELETE_USER])
  async deleteUser(@Args('id') id: string): Promise<BooleanResponse> {
    await this.userService.deleteUser(id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.USER_DELETED'),
      data: true,
    } as BooleanResponse;
  }

  @Mutation(() => AddressResponse)
  @Auth()
  async addAddress(
    @Context() ctx: any,
    @Args('input') input: AddAddressInput,
  ): Promise<AddressResponse> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    const address = await this.userService.addAddress(userId, input);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.ADDRESS_ADDED'),
      data: address,
    } as AddressResponse;
  }

  @Mutation(() => BooleanResponse)
  @Auth()
  async deleteAddress(
    @Context() ctx: any,
    @Args('addressId') addressId: string,
  ): Promise<BooleanResponse> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    await this.userService.deleteAddress(userId, addressId);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.ADDRESS_DELETED'),
      data: true,
    } as BooleanResponse;
  }

  @Query(() => AddressListResponse)
  @Auth()
  async myAddresses(@Context() ctx: any): Promise<AddressListResponse> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    const user = await this.userService.findById(userId);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.ADDRESSES_RETRIEVED'),
      data: user?.addresses || [],
    } as AddressListResponse;
  }

  @Mutation(() => BooleanResponse)
  @Auth()
  async setDefaultAddress(
    @Context() ctx: any,
    @Args('addressId') addressId: string,
  ): Promise<BooleanResponse> {
    const userId = ctx.req.user?.id || ctx.req.headers['x-user-id'];
    await this.userService.setDefaultAddress(userId, addressId);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('user.DEFAULT_ADDRESS_UPDATED'),
      data: true,
    } as BooleanResponse;
  }
}
