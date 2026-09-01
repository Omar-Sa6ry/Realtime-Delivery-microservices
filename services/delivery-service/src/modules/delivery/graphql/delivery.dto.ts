import { Field, InputType } from '@nestjs/graphql';
import { DeliveryStatus } from '../enums/delivery-status.enum';

@InputType()
export class AddressInput {
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

  @Field(() => String, { nullable: true, defaultValue: 'US' })
  countryCode?: string;

  @Field(() => Number, { nullable: true })
  latitude?: number;

  @Field(() => Number, { nullable: true })
  longitude?: number;
}

@InputType()
export class CreateDeliveryInputDto {
  @Field(() => String, { nullable: true })
  customerId?: string;

  @Field(() => String)
  amount: string;

  @Field(() => String, { nullable: true, defaultValue: 'USD' })
  currency?: string;

  @Field(() => AddressInput)
  pickupAddress: AddressInput;

  @Field(() => AddressInput)
  dropoffAddress: AddressInput;

  @Field(() => String, { nullable: true })
  idempotencyKey?: string;
}

@InputType()
export class TransitionDeliveryInput {
  @Field(() => String, { nullable: true })
  deliveryId?: string;

  @Field(() => String, { nullable: true })
  id?: string;

  @Field(() => String)
  status: string;

  @Field(() => String, { nullable: true })
  note?: string;
}
