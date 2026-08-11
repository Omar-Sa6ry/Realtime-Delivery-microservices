import { ObjectType, Field, InputType } from '@nestjs/graphql';
import {
  EmailField,
  PasswordField,
  NameField,
  PhoneField,
  TextField,
} from '@bts-soft/core';
import { GeneralResponse } from '../../../common/graphql/general-response.type';

@ObjectType()
export class UserType {
  @Field(() => String)
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
}

@ObjectType()
export class AuthPayloadType {
  @Field(() => UserType)
  user: UserType;

  @Field(() => String)
  accessToken: string;

  @Field(() => String)
  refreshToken: string;
}

@ObjectType()
export class AuthResponse extends GeneralResponse(AuthPayloadType) {}

@InputType()
export class RegisterInput {
  @EmailField(false, true, false)
  email: string;

  @PasswordField(8, 30, undefined, false, true, false)
  password: string;

  @NameField('firstName', 2, 50, false, true, false)
  firstName: string;

  @NameField('lastName', 2, 50, false, true, false)
  lastName: string;

  @PhoneField('EG', false, true, false)
  phoneNumber?: string;
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
