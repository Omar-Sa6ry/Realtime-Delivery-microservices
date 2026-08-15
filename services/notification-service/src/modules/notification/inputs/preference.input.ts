import { InputType, Field } from '@nestjs/graphql';

@InputType()
export class NotificationPreferenceInput {
  @Field()
  type: string;

  @Field()
  channel: string;

  @Field()
  enabled: boolean;
}
