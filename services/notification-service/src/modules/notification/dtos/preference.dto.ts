import { ObjectType, Field, ID, Directive } from '@nestjs/graphql';
import { NotificationType, NotificationChannel } from '@delivery/common';
import { GeneralResponse } from '@delivery/common';

@Directive('@shareable')
@ObjectType()
export class NotificationPreferenceTypeObj {
  @Field(() => ID)
  id: string;

  @Field(() => NotificationType)
  type: NotificationType;

  @Field(() => NotificationChannel)
  channel: NotificationChannel;

  @Field()
  enabled: boolean;
}

@Directive('@shareable')
@ObjectType()
export class NotificationPreferenceResponse extends GeneralResponse(NotificationPreferenceTypeObj) {}
