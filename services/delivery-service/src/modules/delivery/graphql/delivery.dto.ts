import { Field, InputType } from '@nestjs/graphql';

@InputType()
export class AddressInput {
  @Field() line1!: string;
  @Field({ nullable: true }) line2?: string;
  @Field() city!: string;
  @Field({ nullable: true }) state?: string;
  @Field({ nullable: true }) postalCode?: string;
  @Field({ nullable: true, defaultValue: 'US' }) countryCode?: string;
  @Field({ nullable: true }) latitude?: number;
  @Field({ nullable: true }) longitude?: number;
}

@InputType()
export class CreateDeliveryInputDto {
  @Field({ nullable: true }) customerId?: string;
  @Field() amount!: string;
  @Field({ nullable: true, defaultValue: 'USD' }) currency?: string;
  @Field(() => AddressInput) pickupAddress!: AddressInput;
  @Field(() => AddressInput) dropoffAddress!: AddressInput;
  @Field({ nullable: true }) idempotencyKey?: string;
}

@InputType()
export class TransitionDeliveryInput {
  @Field() deliveryId!: string;
  @Field() status!: string;
  @Field({ nullable: true }) note?: string;
}
