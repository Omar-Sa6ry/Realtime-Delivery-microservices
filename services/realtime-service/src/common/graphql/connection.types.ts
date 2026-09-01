import { ObjectType, Field, Int, Directive, registerEnumType } from '@nestjs/graphql';

export enum PresenceStatus {
  ONLINE = 'ONLINE',
  IDLE = 'IDLE',
  OFFLINE = 'OFFLINE',
}

registerEnumType(PresenceStatus, {
  name: 'PresenceStatus',
  description: 'Presence state of a realtime connection',
});

@Directive('@shareable')
@ObjectType()
export class BooleanResponse {
  @Field(() => String, { nullable: true, defaultValue: 'Operation executed successfully' })
  message?: string = 'Operation executed successfully';

  @Field(() => Boolean, { nullable: true, defaultValue: true })
  success?: boolean = true;

  @Field(() => String, { nullable: true })
  timeStamp?: string = new Date().toISOString();

  @Field(() => Int, { nullable: true, defaultValue: 200 })
  statusCode?: number = 200;

  @Field(() => Boolean, { nullable: true })
  data?: boolean;
}

@Directive('@shareable')
@ObjectType()
export class ConnectionStatusData {
  @Field(() => Boolean)
  isConnected: boolean;

  @Field(() => String, { nullable: true })
  lastSeen?: string;

  @Field(() => Int)
  connectionCount: number;
}

@Directive('@shareable')
@ObjectType()
export class ConnectionsByRole {
  @Field(() => Int)
  customers: number;

  @Field(() => Int)
  drivers: number;

  @Field(() => Int)
  admins: number;
}

@Directive('@shareable')
@ObjectType()
export class ConnectionsCountData {
  @Field(() => Int)
  total: number;

  @Field(() => ConnectionsByRole)
  byRole: ConnectionsByRole;
}

@Directive('@shareable')
@ObjectType()
export class ConnectionStatusResponse {
  @Field(() => String, { nullable: true, defaultValue: 'Operation executed successfully' })
  message?: string = 'Operation executed successfully';

  @Field(() => Boolean, { nullable: true, defaultValue: true })
  success?: boolean = true;

  @Field(() => String, { nullable: true })
  timeStamp?: string = new Date().toISOString();

  @Field(() => Int, { nullable: true, defaultValue: 200 })
  statusCode?: number = 200;

  @Field(() => ConnectionStatusData, { nullable: true })
  data?: ConnectionStatusData;
}

@Directive('@shareable')
@ObjectType()
export class ConnectionsCountResponse {
  @Field(() => String, { nullable: true, defaultValue: 'Operation executed successfully' })
  message?: string = 'Operation executed successfully';

  @Field(() => Boolean, { nullable: true, defaultValue: true })
  success?: boolean = true;

  @Field(() => String, { nullable: true })
  timeStamp?: string = new Date().toISOString();

  @Field(() => Int, { nullable: true, defaultValue: 200 })
  statusCode?: number = 200;

  @Field(() => ConnectionsCountData, { nullable: true })
  data?: ConnectionsCountData;
}