import { ObjectType, Field, ID, Int } from '@nestjs/graphql';
import {
  NotificationType,
  NotificationStatus,
  NotificationPriority,
  NotificationChannel,
  DeliveryChannelStatus,
} from '@delivery/common';
import { GeneralResponse } from '../../../common/graphql/general-response.type';

@ObjectType()
export class NotificationDeliveryType {
  @Field(() => ID)
  id: string;

  @Field()
  channel: NotificationChannel;

  @Field()
  status: DeliveryChannelStatus;

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
  type: NotificationType;

  @Field()
  title: string;

  @Field()
  body: string;

  @Field()
  status: NotificationStatus;

  @Field()
  priority: NotificationPriority;

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

@ObjectType()
export class NotificationResponse extends GeneralResponse(NotificationTypeObj) {}

@ObjectType()
export class PaginatedNotificationResponse extends GeneralResponse(NotificationConnection) {}