import { BadRequestException } from '@nestjs/common';
import { Args, Context, Mutation, Resolver } from '@nestjs/graphql';
import { I18nService } from 'nestjs-i18n';
import { Auth, Permission } from '@delivery/common';
import { RateLimit, RateLimiterAlgorithm } from '@bts-soft/validation';
import { DeliveryCommandService } from '../services/delivery-command.service';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import {
  CreateDeliveryInputDto,
  TransitionDeliveryInput,
} from './delivery.dto';
import { DeliveryResponse } from './delivery.types';
import type { GraphqlContext } from '../../../common/graphql/graphql-context';
import { addressFromInput, deliveryToGraphql } from './delivery.mapper';

@Resolver()
export class DeliveryResolver {
  constructor(
    private readonly commands: DeliveryCommandService,
    private readonly i18n: I18nService,
  ) {}

  @Auth([Permission.CREATE_DELIVERY])
  @RateLimit({ algorithm: RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER, limit: 10, windowMs: 60000 })
  @Mutation(() => DeliveryResponse)
  async createDelivery(
    @Args('input') input: CreateDeliveryInputDto,
    @Context() ctx: GraphqlContext,
  ): Promise<DeliveryResponse> {
    const customerId =
      input.customerId ?? ctx.req?.user?.id ?? ctx.req?.headers?.['x-user-id'];
    if (!customerId)
      throw new BadRequestException(
        await this.i18n.t('delivery.customerIdRequired', {
          lang: ctx.language,
        }),
      );
    const delivery = await this.commands.create({
      customerId,
      amount: input.amount,
      currency: input.currency,
      pickupAddress: addressFromInput(input.pickupAddress),
      dropoffAddress: addressFromInput(input.dropoffAddress),
      idempotencyKey: input.idempotencyKey,
    });
    return {
      success: true,
      statusCode: 201,
      message: await this.i18n.t('delivery.created', { lang: ctx.language }),
      data: deliveryToGraphql(delivery),
    };
  }

  @Auth([Permission.UPDATE_DELIVERY_STATUS])
  @RateLimit({ algorithm: RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER, limit: 30, windowMs: 60000 })
  @Auth([Permission.CANCEL_DELIVERY])
  @RateLimit({ algorithm: RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER, limit: 10, windowMs: 60000 })
  @Auth([Permission.UPDATE_DELIVERY_STATUS])
  @RateLimit({ algorithm: RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER, limit: 30, windowMs: 60000 })
  @Mutation(() => DeliveryResponse)
  async transitionDelivery(
    @Args('input') input: TransitionDeliveryInput,
    @Context() ctx: GraphqlContext,
  ): Promise<DeliveryResponse> {
    const delivery = await this.commands.transition(
      input.deliveryId,
      input.status as DeliveryStatus,
      ctx.req?.user?.id,
      input.note,
    );
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('delivery.statusUpdated', {
        lang: ctx.language,
      }),
      data: deliveryToGraphql(delivery),
    };
  }
  
  @Auth([Permission.CANCEL_DELIVERY])
  @RateLimit({ algorithm: RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER, limit: 10, windowMs: 60000 })
  @Mutation(() => DeliveryResponse)
  async cancelDelivery(
    @Args('deliveryId') deliveryId: string,
    @Context() ctx: GraphqlContext,
  ): Promise<DeliveryResponse> {
    const delivery = await this.commands.cancel(deliveryId, ctx.req?.user?.id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('delivery.cancelled', { lang: ctx.language }),
      data: deliveryToGraphql(delivery),
    };
  }
}




