import { BadRequestException, ForbiddenException, Logger } from '@nestjs/common';
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
import { DeliverySagaOrchestrator } from '../saga/delivery-saga.orchestrator';

@Resolver(() => DeliveryType)
export class DeliveryResolver {
  private readonly logger = new Logger(DeliveryResolver.name);

  constructor(
    private readonly commands: DeliveryCommandService,
    private readonly queries: DeliveryQueryService,
    private readonly i18n: I18nService,
    private readonly saga: DeliverySagaOrchestrator,
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
    const tokenUserId = ctx.req?.user?.id ?? ctx.req?.headers?.['x-user-id'];
    const userRole = (ctx.req?.user as any)?.role ?? ctx.req?.headers?.['x-user-role'];

    let customerId = tokenUserId;
    if (userRole === 'ADMIN' || userRole === 'admin') {
      customerId = input.customerId ?? tokenUserId;
    }

    if (!customerId) {
      throw new BadRequestException(
        await this.i18n.t('delivery.customerIdRequired', {
          lang: ctx.language,
        }),
      );
    }

    const delivery = await this.commands.create({
      customerId,
      amount: input.amount,
      currency: input.currency,
      pickupAddress: addressFromInput(input.pickupAddress),
      dropoffAddress: addressFromInput(input.dropoffAddress),
      idempotencyKey: input.idempotencyKey,
    });

    // Run saga workflow asynchronously in the background
    setImmediate(() => {
      this.saga.execute(delivery.id).catch((err: Error) => {
        this.logger.error(`Saga failed for delivery [${delivery.id}]: ${err.message}`);
      });
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

    const tokenUserId = ctx?.req?.user?.id ?? ctx?.req?.headers?.['x-user-id'];
    const userRole = (ctx?.req?.user as any)?.role ?? ctx?.req?.headers?.['x-user-role'];
    const isAdmin = userRole === 'ADMIN' || userRole === 'admin';

    const existing = await this.queries.getById(targetId as string);
    if (!isAdmin && existing.customerId !== tokenUserId) {
      throw new ForbiddenException(
        await this.i18n.t('delivery.unauthorizedCancel', { lang: ctx?.language }),
      );
    }

    const delivery = await this.commands.cancel(targetId as string, tokenUserId, reason);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('delivery.cancelled', { lang: ctx?.language }),
      data: deliveryToGraphql(delivery),
    };
  }
}
