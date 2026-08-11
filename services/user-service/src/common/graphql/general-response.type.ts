import { ObjectType, Field } from '@nestjs/graphql';
import { Type } from '@nestjs/common';
import { GraphqlBaseResponse } from '@bts-soft/core';

export function GeneralResponse<T>(classRef: Type<T>): Type<any> {
  @ObjectType({ isAbstract: true })
  abstract class GeneralResponseClass extends GraphqlBaseResponse {
    @Field(() => classRef, { nullable: true })
    data?: T;

    @Field(() => [classRef], { nullable: true })
    items?: T[];
  }
  return GeneralResponseClass as Type<any>;
}

@ObjectType()
export class BooleanResponse extends GraphqlBaseResponse {
  @Field(() => Boolean, { nullable: true })
  data?: boolean;
}
