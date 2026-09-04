import { BadRequestException, ForbiddenException } from '@nestjs/common';
import { Args, Context, Int, Query, Resolver } from '@nestjs/graphql';
import { I18nService } from 'nestjs-i18n';
import { Auth, Permission } from '@delivery/common';
import { RateLimit, RateLimiterAlgorithm } from '@bts-soft/validation';
import { DeliveryQueryService } from '../services/delivery-query.service';
import {
  DeliveryType,
  DeliveryResponse,
  DeliveryListResponse,
  DeliveryStatusesResponse,
} from './delivery.types';
import type { GraphqlContext } from '../../../common/graphql/graphql-context';
import { deliveryToGraphql } from './delivery.mapper';

@Resolver(() => DeliveryType)
export class DeliveryQueryResolver {
  constructor(
    private readonly queries: DeliveryQueryService,
    private readonly i18n: I18nService,
  ) {}

  @Auth([Permission.VIEW_DELIVERY])
  @RateLimit({ algorithm: RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER, limit: 60, windowMs: 60000 })
  @Query(() => DeliveryResponse)
  async delivery(
    @Args('id') id: string,
    @Context() ctx: GraphqlContext,
  ): Promise<DeliveryResponse> {
    const delivery = await this.queries.getById(id);
    const tokenUserId = ctx.req?.user?.id ?? ctx.req?.headers?.['x-user-id'];
    const userRole = (ctx.req?.user as any)?.role ?? ctx.req?.headers?.['x-user-role'];
    const isAdmin = userRole === 'ADMIN' || userRole === 'admin';

    if (!isAdmin && delivery.customerId !== tokenUserId) {
      throw new ForbiddenException(
        await this.i18n.t('delivery.unauthorizedAccess', { lang: ctx.language }),
      );
    }

    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('delivery.retrieved', { lang: ctx.language }),
      data: deliveryToGraphql(delivery),
    };
  }

  @Auth([Permission.VIEW_DELIVERY])
  @RateLimit({ algorithm: RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER, limit: 60, windowMs: 60000 })
  @Query(() => DeliveryListResponse)
  async myDeliveries(
    @Args({ name: 'page', type: () => Int, nullable: true, defaultValue: 1 })
    page: number,
    @Args({
      name: 'pageSize',
      type: () => Int,
      nullable: true,
      defaultValue: 50,
    })
    pageSize: number,
    @Context() ctx: GraphqlContext,
  ): Promise<DeliveryListResponse> {
    const customerId = ctx.req?.user?.id ?? ctx.req?.headers?.['x-user-id'];
    if (!customerId)
      throw new BadRequestException(
        await this.i18n.t('delivery.authenticatedCustomerRequired', {
          lang: ctx.language,
        }),
      );
    const [deliveries, total] = await this.queries.listByCustomer(
      customerId,
      page,
      pageSize,
    );
    const totalPages = Math.ceil(total / pageSize);
    const nextPage = page < totalPages ? page + 1 : null;
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('delivery.retrieved', { lang: ctx.language }),
      data: {
        items: deliveries.map(deliveryToGraphql),
        paginationInfo: {
          totalItems: total,
          currentPage: page,
          nextPage,
        },
      },
    } as DeliveryListResponse;
  }

  @Auth([Permission.VIEW_DELIVERY])
  @RateLimit({ algorithm: RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER, limit: 120, windowMs: 60000 })
  @Query(() => DeliveryStatusesResponse)
  async deliveryNextStatuses(
    @Args('id') id: string,
    @Context() ctx: GraphqlContext,
  ): Promise<DeliveryStatusesResponse> {
    const delivery = await this.queries.getById(id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('delivery.nextStatusesRetrieved', {
        lang: ctx.language,
      }),
      data: {
        statuses: this.queries.nextStatuses(delivery),
      },
    } as DeliveryStatusesResponse;
  }
}
