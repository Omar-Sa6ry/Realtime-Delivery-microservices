import { ObjectType, Field, ID, Int } from '@nestjs/graphql';
import { NotificationType, NotificationStatus, NotificationPriority } from '@delivery/common';

@ObjectType()
export class NotificationDeliveryType {
  @Field(() => ID)
  id: string;

  @Field()
  channel: string;

  @Field()
  status: string;

  @Field({ nullable: true })
  sentAt?: Date;

  @Field({ nullable: true })
  deliveredAt?: Date;
}

@ObjectType()
export class NotificationTypeObj {
  @Field(() => ID)
  id: string;

  @Field()
  type: string;

  @Field()
  title: string;

  @Field()
  body: string;

  @Field()
  status: string;

  @Field()
  priority: string;

  @Field({ nullable: true })
  readAt?: Date;

  @Field()
  createdAt: Date;

  @Field(() => [NotificationDeliveryType])
  deliveries: NotificationDeliveryType[];
}

@ObjectType()
export class NotificationConnection {
  @Field(() => [NotificationTypeObj])
  items: NotificationTypeObj[];

  @Field(() => Int)
  totalCount: number;
}
