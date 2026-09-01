import { ObjectType, Field, ID, Directive } from '@nestjs/graphql';
import { NotificationType, NotificationChannel } from '@delivery/common';
import { GeneralResponse } from '../../../common/graphql/general-response.type';

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