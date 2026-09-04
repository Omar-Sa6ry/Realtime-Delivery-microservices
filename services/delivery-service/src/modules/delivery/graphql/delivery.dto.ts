import { Field, InputType } from '@nestjs/graphql';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { IsString, IsOptional, IsNumber, ValidateNested, IsNotEmpty, IsEnum } from 'class-validator';
import { Type } from 'class-transformer';

@InputType()
export class AddressInput {
  @Field(() => String)
  @IsString()
  @IsNotEmpty()
  line1: string;

  @Field(() => String, { nullable: true })
  @IsString()
  @IsOptional()
  line2?: string;

  @Field(() => String)
  @IsString()
  @IsNotEmpty()
  city: string;

  @Field(() => String, { nullable: true })
  @IsString()
  @IsOptional()
  state?: string;

  @Field(() => String, { nullable: true })
  @IsString()
  @IsOptional()
  postalCode?: string;

  @Field(() => String, { nullable: true, defaultValue: 'US' })
  @IsString()
  @IsOptional()
  countryCode?: string;

  @Field(() => Number, { nullable: true })
  @IsNumber()
  @IsOptional()
  latitude?: number;

  @Field(() => Number, { nullable: true })
  @IsNumber()
  @IsOptional()
  longitude?: number;
}

@InputType()
export class CreateDeliveryInputDto {
  @Field(() => String, { nullable: true })
  @IsString()
  @IsOptional()
  customerId?: string;

  @Field(() => String)
  @IsString()
  @IsNotEmpty()
  amount: string;

  @Field(() => String, { nullable: true, defaultValue: 'USD' })
  @IsString()
  @IsOptional()
  currency?: string;

  @Field(() => AddressInput)
  @ValidateNested()
  @Type(() => AddressInput)
  @IsNotEmpty()
  pickupAddress: AddressInput;

  @Field(() => AddressInput)
  @ValidateNested()
  @Type(() => AddressInput)
  @IsNotEmpty()
  dropoffAddress: AddressInput;

  @Field(() => String, { nullable: true })
  @IsString()
  @IsOptional()
  idempotencyKey?: string;
}

@InputType()
export class TransitionDeliveryInput {
  @Field(() => String, { nullable: true })
  @IsString()
  @IsOptional()
  deliveryId?: string;

  @Field(() => String, { nullable: true })
  @IsString()
  @IsOptional()
  id?: string;

  @Field(() => DeliveryStatus)
  @IsEnum(DeliveryStatus)
  status: DeliveryStatus;

  @Field(() => String, { nullable: true })
  @IsString()
  @IsOptional()
  note?: string;
}
