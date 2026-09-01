import { ObjectType, Field, InputType, Directive, ID } from '@nestjs/graphql';
import { PasswordField, PhoneField, TextField } from '@bts-soft/core';
import { GeneralResponse } from '../../../common/graphql/general-response.type';
import { IsBoolean, IsOptional, IsNumber, IsString } from 'class-validator';

@Directive('@key(fields: "id")')
@ObjectType()
export class AddressType {
  @Field(() => ID)
  id: string;

  @Field(() => String)
  userId: string;

  @Field(() => String)
  title: string;

  @Field(() => String)
  street: string;

  @Field(() => String)
  city: string;

  @Field(() => String, { nullable: true })
  state?: string;

  @Field(() => String, { nullable: true })
  postalCode?: string;

  @Field(() => Number, { nullable: true })
  latitude?: number;

  @Field(() => Number, { nullable: true })
  longitude?: number;

  @Field(() => Boolean)
  isDefault: boolean;

  @Field(() => Date)
  createdAt: Date;
}

@Directive('@key(fields: "id")')
@ObjectType()
export class UserType {
  @Field(() => ID)
  id: string;

  @Field(() => String)
  email: string;

  @Field(() => String)
  firstName: string;

  @Field(() => String)
  lastName: string;

  @Field(() => String)
  role: string;

  @Field(() => String, { nullable: true })
  phoneNumber?: string;

  @Field(() => Boolean)
  isActive: boolean;

  @Field(() => Date)
  createdAt: Date;

  @Field(() => String, { nullable: true })
  imageUrl?: string;

  @Field(() => [AddressType])
  addresses: AddressType[];
}

@Directive('@shareable')
@ObjectType()
export class UserResponse extends GeneralResponse(UserType) {}

@InputType()
export class ChangePasswordInput {
  @PasswordField(8, 30, undefined, false, true, false)
  passwordOld: string;

  @PasswordField(8, 30, undefined, false, true, false)
  passwordNew: string;
}

@InputType()
export class UpdateProfileInput {
  @TextField('firstName', 2, 100, true, true, true)
  firstName?: string;

  @TextField('lastName', 2, 100, true, true, true)
  lastName?: string;

  @Field(() => String, { nullable: true })
  @IsOptional()
  @IsString()
  imageUrl?: string;

  @Field(() => String, { nullable: true })
  @IsOptional()
  @IsString()
  avatarMediaId?: string;
}

@Directive('@shareable')
@ObjectType()
export class PaginatedUsers {
  @Field(() => [UserType])
  items: UserType[];

  @Field(() => Number)
  total: number;
}

@Directive('@shareable')
@ObjectType()
export class PaginatedUsersResponse extends GeneralResponse(PaginatedUsers) {}

@InputType()
export class AddAddressInput {
  @TextField('title', 2, 100, false, true, false)
  title: string;

  @TextField('street', 2, 200, false, true, false)
  street: string;

  @TextField('city', 2, 100, false, true, false)
  city: string;

  @TextField('state', 2, 100, true, true, true)
  state?: string;

  @TextField('postalCode', 2, 20, true, true, true)
  postalCode?: string;

  @Field(() => Number, { nullable: true })
  @IsOptional()
  @IsNumber()
  latitude?: number;

  @Field(() => Number, { nullable: true })
  @IsOptional()
  @IsNumber()
  longitude?: number;

  @Field(() => Boolean, { nullable: true })
  @IsOptional()
  @IsBoolean()
  isDefault?: boolean;
}

@Directive('@shareable')
@ObjectType()
export class AddressResponse extends GeneralResponse(AddressType) {}

@Directive('@shareable')
@ObjectType()
export class AddressListResponse extends GeneralResponse(AddressType) {
  @Field(() => [AddressType], { nullable: true })
  data?: AddressType[];
}
