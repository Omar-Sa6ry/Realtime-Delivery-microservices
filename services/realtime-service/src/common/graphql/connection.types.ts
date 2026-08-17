import { ObjectType, Field, Int, Directive, registerEnumType } from '@nestjs/graphql';
import { GraphqlBaseResponse } from '@bts-soft/core';

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
export class BooleanResponse extends GraphqlBaseResponse {
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

@ObjectType()
export class ConnectionStatusResponse extends GraphqlBaseResponse {
  @Field(() => ConnectionStatusData, { nullable: true })
  data?: ConnectionStatusData;
}

@ObjectType()
export class ConnectionsCountResponse extends GraphqlBaseResponse {
  @Field(() => ConnectionsCountData, { nullable: true })
  data?: ConnectionsCountData;
}