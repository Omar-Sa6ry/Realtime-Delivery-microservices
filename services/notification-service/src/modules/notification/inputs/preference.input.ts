import { InputType, Field } from '@nestjs/graphql';
import { NotificationType, NotificationChannel } from '@delivery/common';

@InputType()
export class ChannelPreferenceInput {
  @Field(() => NotificationChannel)
  channel: NotificationChannel;

  @Field()
  enabled: boolean;
}

@InputType()
export class NotificationPreferenceInput {
  @Field(() => NotificationType)
  type: NotificationType;

  @Field(() => [ChannelPreferenceInput])
  channels: ChannelPreferenceInput[];
}
