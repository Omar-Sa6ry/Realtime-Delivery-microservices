import { Field, ObjectType, Int } from '@nestjs/graphql';
import { DeliveryStatus } from '../enums/delivery-status.enum';

@ObjectType()
export class AddressType {
  @Field() line1!: string;
  @Field({ nullable: true }) line2?: string;
  @Field() city!: string;
  @Field({ nullable: true }) state?: string;
  @Field({ nullable: true }) postalCode?: string;
  @Field() countryCode!: string;
  @Field({ nullable: true }) latitude?: number;
  @Field({ nullable: true }) longitude?: number;
}

@ObjectType()
export class DeliveryType {
  @Field() id!: string;
  @Field() customerId!: string;
  @Field({ nullable: true }) driverId?: string;
  @Field(() => DeliveryStatus) status!: DeliveryStatus;
  @Field() paymentStatus!: string;
  @Field() amount!: string;
  @Field() currency!: string;
  @Field(() => AddressType) pickupAddress!: AddressType;
  @Field(() => AddressType) dropoffAddress!: AddressType;
  @Field({ nullable: true }) pickedUpAt?: Date;
  @Field({ nullable: true }) completedAt?: Date;
  @Field({ nullable: true }) cancelledAt?: Date;
  @Field() createdAt!: Date;
  @Field() updatedAt!: Date;
}

@ObjectType()
export class DeliveryResponse {
  @Field() success!: boolean;
  @Field(() => Int) statusCode!: number;
  @Field() message!: string;
  @Field(() => DeliveryType, { nullable: true }) data?: DeliveryType;
}

@ObjectType()
export class DeliveryListResponse {
  @Field() success!: boolean;
  @Field(() => Int) statusCode!: number;
  @Field() message!: string;
  @Field(() => [DeliveryType]) data!: DeliveryType[];
  @Field(() => Int) total!: number;
}

@ObjectType()
export class DeliveryStatusesResponse {
  @Field() success!: boolean;
  @Field(() => Int) statusCode!: number;
  @Field() message!: string;
  @Field(() => [DeliveryStatus]) data!: DeliveryStatus[];
}
