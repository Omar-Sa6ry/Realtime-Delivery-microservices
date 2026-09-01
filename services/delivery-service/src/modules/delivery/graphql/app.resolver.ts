import { Query, Resolver, Context } from '@nestjs/graphql';
import { I18nService } from 'nestjs-i18n';
import { DeliveryServiceInfoResponse } from './delivery.types';
import type { GraphqlContext } from '../../../common/graphql/graphql-context';

@Resolver()
export class AppResolver {
  constructor(private readonly i18n: I18nService) {}

  @Query(() => DeliveryServiceInfoResponse)
  async deliveryServiceInfo(
    @Context() ctx?: GraphqlContext,
  ): Promise<DeliveryServiceInfoResponse> {
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('delivery.serviceInfo', {
        lang: ctx?.language ?? 'en',
      }),
      data: {
        name: 'delivery-service',
        version: process.env.npm_package_version ?? '0.0.1',
        status: 'ok',
      },
    };
  }
}

