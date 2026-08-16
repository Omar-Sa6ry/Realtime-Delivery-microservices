import { ObjectType, Field, ID } from '@nestjs/graphql';
import { NotificationType, NotificationChannel } from '@delivery/common';
import { GeneralResponse } from '../../../common/graphql/general-response.type';

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

@ObjectType()
export class NotificationPreferenceResponse extends GeneralResponse(NotificationPreferenceTypeObj) {}