import { ObjectType, Field, ID, Int, Directive, registerEnumType } from '@nestjs/graphql';
import {
  NotificationType,
  NotificationStatus,
  NotificationPriority,
  NotificationChannel,
  DeliveryChannelStatus,
} from '@delivery/common';
import { GeneralResponse } from '@delivery/common';

registerEnumType(NotificationType, {
  name: 'NotificationType',
  description: 'Types of notifications supported',
});

registerEnumType(NotificationStatus, {
  name: 'NotificationStatus',
  description: 'Status of a notification',
});

registerEnumType(NotificationPriority, {
  name: 'NotificationPriority',
  description: 'Priority level of a notification',
});

registerEnumType(NotificationChannel, {
  name: 'NotificationChannel',
  description: 'Delivery channel for notifications',
});

registerEnumType(DeliveryChannelStatus, {
  name: 'DeliveryChannelStatus',
  description: 'Delivery status for a specific channel',
});

@Directive('@shareable')
@ObjectType()
export class NotificationDeliveryType {
  @Field(() => ID)
  id: string;

  @Field(() => NotificationChannel)
  channel: NotificationChannel;

  @Field(() => DeliveryChannelStatus)
  status: DeliveryChannelStatus;

  @Field(() => Date, { nullable: true })
  sentAt?: Date;

  @Field(() => Date, { nullable: true })
  deliveredAt?: Date;
}

@Directive('@key(fields: "id")')
@ObjectType()
export class NotificationTypeObj {
  @Field(() => ID)
  id: string;

  @Field(() => NotificationType)
  type: NotificationType;

  @Field(() => String)
  title: string;

  @Field(() => String)
  body: string;

  @Field(() => NotificationStatus)
  status: NotificationStatus;

  @Field(() => NotificationPriority)
  priority: NotificationPriority;

  @Field(() => Date, { nullable: true })
  readAt?: Date;

  @Field(() => Date)
  createdAt: Date;

  @Field(() => [NotificationDeliveryType])
  deliveries: NotificationDeliveryType[];
}

@Directive('@shareable')
@ObjectType()
export class NotificationConnection {
  @Field(() => [NotificationTypeObj])
  items: NotificationTypeObj[];

  @Field(() => Int)
  totalCount: number;
}

@Directive('@shareable')
@ObjectType()
export class NotificationResponse extends GeneralResponse(NotificationTypeObj) {}

@Directive('@shareable')
@ObjectType()
export class PaginatedNotificationResponse extends GeneralResponse(NotificationConnection) {}
