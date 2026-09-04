import { Field, ObjectType, Directive, ID, registerEnumType } from '@nestjs/graphql';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { PaymentStatus } from '../enums/payment-status.enum';
import { GeneralResponse } from '@delivery/common';

registerEnumType(DeliveryStatus, {
  name: 'DeliveryStatus',
  description: 'The lifecycle status of a delivery',
});

registerEnumType(PaymentStatus, {
  name: 'PaymentStatus',
  description: 'The payment status of a delivery',
});

@Directive('@key(fields: "id")')
@Directive('@shareable')
@ObjectType()
export class UserType {
  @Field(() => ID)
  id: string;
}

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
  @Field(() => DeliveryStatus)
  status: DeliveryStatus;

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

  @Field(() => UserType, { nullable: true })
  customer?: UserType;

  @Field(() => UserType, { nullable: true })
  driver?: UserType;

  @Field(() => DeliveryStatus)
  status: DeliveryStatus;

  @Field(() => PaymentStatus)
  paymentStatus: PaymentStatus;

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
export class PaginationInfo {
  @Field(() => Number)
  totalItems: number;

  @Field(() => Number)
  currentPage: number;

  @Field(() => Number, { nullable: true })
  nextPage?: number | null;
}

@Directive('@shareable')
@ObjectType()
export class PaginatedDeliveries {
  @Field(() => [DeliveryType])
  items: DeliveryType[];

  @Field(() => PaginationInfo)
  paginationInfo: PaginationInfo;
}

@Directive('@shareable')
@ObjectType()
export class DeliveryListResponse extends GeneralResponse(PaginatedDeliveries) {}

@Directive('@shareable')
@ObjectType()
export class DeliveryStatusesData {
  @Field(() => [DeliveryStatus])
  statuses: DeliveryStatus[];
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



