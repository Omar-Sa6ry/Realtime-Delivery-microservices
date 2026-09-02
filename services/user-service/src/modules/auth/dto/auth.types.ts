import { ObjectType, Field, InputType, Directive } from '@nestjs/graphql';
import {
  EmailField,
  PasswordField,
  NameField,
  PhoneField,
  TextField,
} from '@bts-soft/core';
import { IsOptional, IsString } from 'class-validator';
import { GeneralResponse } from '@delivery/common';

import { UserType } from '../../user/dto/user.types';

@Directive('@shareable')
@ObjectType()
export class AuthPayloadType {
  @Field(() => UserType)
  user: UserType;

  @Field(() => String)
  accessToken: string;

  @Field(() => String)
  refreshToken: string;
}

@Directive('@shareable')
@ObjectType()
export class AuthResponse extends GeneralResponse(AuthPayloadType) {}

@InputType()
export class RegisterInput {
  @EmailField(false, true, false)
  email: string;

  @PasswordField(8, 30, undefined, false, true, false)
  password: string;

  @NameField('firstName', 2, 100, false, true, false)
  firstName: string;

  @NameField('lastName', 2, 100, false, true, false)
  lastName: string;

  @PhoneField('EG', false, true, false)
  phoneNumber?: string;

  @Field(() => String, { nullable: true })
  @IsOptional()
  @IsString()
  imageUrl?: string;
}

@InputType()
export class LoginInput {
  @EmailField(false, true, false)
  email: string;

  @PasswordField(8, 30, undefined, false, true, false)
  password: string;
}

@InputType()
export class ForgetPasswordInput {
  @EmailField(false, true, false)
  email: string;
}

@InputType()
export class ResetPasswordInput {
  @TextField('token', 1, 100, false, true, false)
  token: string;

  @PasswordField(8, 30, undefined, false, true, false)
  passwordNew: string;
}

@InputType()
export class RefreshTokenInput {
  @TextField('refreshToken', 10, 1000, false, true, false)
  refreshToken: string;
}

