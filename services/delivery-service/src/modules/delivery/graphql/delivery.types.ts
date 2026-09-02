import { Field, ObjectType, Directive, ID } from '@nestjs/graphql';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { GeneralResponse } from '@delivery/common';

@Directive('@shareable')
@ObjectType()
export class DeliveryAddressType {
  @Field(() => String)
  line1: string;

  @Field(() => String, { nullable: true })
  line2?: string;

  @Field(() => String)
  city: string;

  @Field(() => String, { nullable: true })
  state?: string;

  @Field(() => String, { nullable: true })
  postalCode?: string;

  @Field(() => String)
  countryCode: string;

  @Field(() => Number, { nullable: true })
  latitude?: number;

  @Field(() => Number, { nullable: true })
  longitude?: number;
}

@Directive('@shareable')
@ObjectType()
export class DeliveryStatusHistoryType {
  @Field(() => String)
  status: string;

  @Field(() => String, { nullable: true })
  changedBy?: string;

  @Field(() => String, { nullable: true })
  note?: string;

  @Field(() => Date)
  createdAt: Date;
}

@Directive('@key(fields: "id")')
@ObjectType()
export class DeliveryType {
  @Field(() => ID)
  id: string;

  @Field(() => String)
  customerId: string;

  @Field(() => String, { nullable: true })
  driverId?: string;

  @Field(() => String)
  status: string;

  @Field(() => String)
  paymentStatus: string;

  @Field(() => String)
  amount: string;

  @Field(() => String)
  currency: string;

  @Field(() => DeliveryAddressType)
  pickupAddress: DeliveryAddressType;

  @Field(() => DeliveryAddressType)
  dropoffAddress: DeliveryAddressType;

  @Field(() => [DeliveryStatusHistoryType], { nullable: true })
  statusHistory?: DeliveryStatusHistoryType[];

  @Field(() => Date, { nullable: true })
  pickedUpAt?: Date;

  @Field(() => Date, { nullable: true })
  completedAt?: Date;

  @Field(() => Date, { nullable: true })
  cancelledAt?: Date;

  @Field(() => Date)
  createdAt: Date;

  @Field(() => Date)
  updatedAt: Date;
}

@Directive('@shareable')
@ObjectType()
export class DeliveryResponse extends GeneralResponse(DeliveryType) {}

@Directive('@shareable')
@ObjectType()
export class PaginatedDeliveries {
  @Field(() => [DeliveryType])
  items: DeliveryType[];

  @Field(() => Number)
  total: number;
}

@Directive('@shareable')
@ObjectType()
export class DeliveryListResponse extends GeneralResponse(PaginatedDeliveries) {}

@Directive('@shareable')
@ObjectType()
export class DeliveryStatusesData {
  @Field(() => [String])
  statuses: string[];
}

@Directive('@shareable')
@ObjectType()
export class DeliveryStatusesResponse extends GeneralResponse(DeliveryStatusesData) {}

@Directive('@shareable')
@ObjectType()
export class DeliveryServiceInfo {
  @Field(() => String)
  name: string;

  @Field(() => String)
  version: string;

  @Field(() => String)
  status: string;
}

@Directive('@shareable')
@ObjectType()
export class DeliveryServiceInfoResponse extends GeneralResponse(DeliveryServiceInfo) {}


