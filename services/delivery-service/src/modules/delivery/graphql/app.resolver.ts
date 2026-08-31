import { Query, Resolver, Context, Directive, Field, ObjectType } from '@nestjs/graphql';
import { I18nService } from 'nestjs-i18n';

@Directive('@shareable')
@ObjectType()
class DeliveryServiceInfo {
  @Field(() => String)
  name: string;

  @Field(() => String)
  version: string;

  @Field(() => String)
  status: string;
}

@Resolver()
export class AppResolver {
  constructor(private readonly i18n: I18nService) {}
  @Query(() => DeliveryServiceInfo)
  deliveryServiceInfo(): DeliveryServiceInfo {
    return {
      name: 'delivery-service',
      version: process.env.npm_package_version ?? '0.0.1',
      status: 'ok',
    };
  }
}

