import { Field, ObjectType } from "@nestjs/graphql";
import { Exclude } from "class-transformer";
import { Role } from "../constants/enum.constant";

@ObjectType()
export class UserDto {
  @Field(() => String)
  id: string;

  @Field(() => String)
  email: string;

  @Field(() => String)
  firstName: string;

  @Field(() => String)
  lastName: string;

  @Field(() => String)
  role: Role;

  @Field(() => String, { nullable: true })
  phoneNumber?: string;

  @Field(() => Boolean)
  isActive: boolean;

  @Field(() => Boolean)
  isVerified: boolean;

  @Exclude()
  passwordHash?: string;

  @Field(() => Date)
  createdAt: Date;

  @Field(() => Date)
  updatedAt: Date;
}
