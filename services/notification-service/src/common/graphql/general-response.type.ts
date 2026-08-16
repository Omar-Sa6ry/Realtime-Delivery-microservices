import { ObjectType, Field, Int, Directive } from '@nestjs/graphql';
import { Type } from '@nestjs/common';
import { GraphqlBaseResponse } from '@bts-soft/core';

export function GeneralResponse<T>(classRef: Type<T>) {
  @ObjectType({ isAbstract: true })
  abstract class GeneralResponseClass extends GraphqlBaseResponse {
    @Field(() => classRef, { nullable: true })
    data?: T;

    @Field(() => [classRef], { nullable: true })
    items?: T[];
  }
  return GeneralResponseClass;
}

@Directive('@shareable')
@ObjectType()
export class BooleanResponse extends GraphqlBaseResponse {
  @Field(() => Boolean, { nullable: true })
  data?: boolean;
}

@ObjectType()
export class IntResponse extends GraphqlBaseResponse {
  @Field(() => Int, { nullable: true })
  data?: number;
}