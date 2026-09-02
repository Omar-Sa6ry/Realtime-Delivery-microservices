import { ObjectType, Field, Directive, Int } from '@nestjs/graphql';
import { Type } from '@nestjs/common';

export function GeneralResponse<T>(classRef: Type<T>): any {
  @Directive('@shareable')
  @ObjectType({ isAbstract: true })
  abstract class GeneralResponseClass {
    @Field(() => String, { nullable: true, defaultValue: 'Operation executed successfully' })
    message?: string = 'Operation executed successfully';

    @Field(() => Boolean, { nullable: true, defaultValue: true })
    success?: boolean = true;

    @Field(() => String, { nullable: true })
    timeStamp?: string = new Date().toISOString();

    @Field(() => Int, { nullable: true, defaultValue: 200 })
    statusCode?: number = 200;

    @Field(() => classRef, { nullable: true })
    data?: T;

    @Field(() => [classRef], { nullable: true })
    items?: T[];
  }
  return GeneralResponseClass;
}

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
export class IntResponse {
  @Field(() => String, { nullable: true, defaultValue: 'Operation executed successfully' })
  message?: string = 'Operation executed successfully';

  @Field(() => Boolean, { nullable: true, defaultValue: true })
  success?: boolean = true;

  @Field(() => String, { nullable: true })
  timeStamp?: string = new Date().toISOString();

  @Field(() => Int, { nullable: true, defaultValue: 200 })
  statusCode?: number = 200;

  @Field(() => Int, { nullable: true })
  data?: number;
}
