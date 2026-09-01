import { BadRequestException } from '@nestjs/common';
import { Args, Context, Mutation, Resolver, ResolveReference } from '@nestjs/graphql';
import { I18nService } from 'nestjs-i18n';
import { Auth, Permission } from '@delivery/common';
import { DeliveryCommandService } from '../services/delivery-command.service';
import { DeliveryQueryService } from '../services/delivery-query.service';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import {
  CreateDeliveryInputDto,
  TransitionDeliveryInput,
} from './delivery.dto';
import { DeliveryResponse, DeliveryType } from './delivery.types';
import type { GraphqlContext } from '../../../common/graphql/graphql-context';
import { addressFromInput, deliveryToGraphql } from './delivery.mapper';

@Resolver(() => DeliveryType)
export class DeliveryResolver {
  constructor(
    private readonly commands: DeliveryCommandService,
    private readonly queries: DeliveryQueryService,
    private readonly i18n: I18nService,
  ) {}

  @ResolveReference()
  async resolveReference(reference: { __typename: string; id: string }): Promise<DeliveryType> {
    return deliveryToGraphql(await this.queries.getById(reference.id));
  }

  @Auth([Permission.CREATE_DELIVERY])
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
  @Mutation(() => DeliveryResponse)
  async transitionDelivery(
    @Args('input') input: TransitionDeliveryInput,
    @Context() ctx: GraphqlContext,
  ): Promise<DeliveryResponse> {
    const targetId = input.deliveryId ?? input.id;
    if (!targetId) {
      throw new BadRequestException('deliveryId or id is required');
    }
    const delivery = await this.commands.transition(
      targetId as string,
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
  @Mutation(() => DeliveryResponse)
  async cancelDelivery(
    @Args({ name: 'deliveryId', type: () => String, nullable: true }) deliveryIdArg?: string,
    @Args({ name: 'id', type: () => String, nullable: true }) idArg?: string,
    @Args({ name: 'reason', type: () => String, nullable: true }) reason?: string,
    @Context() ctx?: GraphqlContext,
  ): Promise<DeliveryResponse> {
    const targetId = deliveryIdArg ?? idArg;
    if (!targetId) {
      throw new BadRequestException('deliveryId or id is required');
    }
    const delivery = await this.commands.cancel(targetId as string, ctx?.req?.user?.id, reason);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('delivery.cancelled', { lang: ctx?.language }),
      data: deliveryToGraphql(delivery),
    };
  }
}




