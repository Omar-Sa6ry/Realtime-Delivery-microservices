import { Inject, Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { lastValueFrom } from 'rxjs';
import { DeliveryStatus } from '../../enums/delivery-status.enum';
import { PaymentStatus } from '../../enums/payment-status.enum';
import { DeliveryCommandService } from '../../services/delivery-command.service';
import { DeliverySagaContext, DeliverySagaStep } from '../saga-step';

@Injectable()
export class PaymentConfirmationStep implements DeliverySagaStep, OnModuleInit {
  readonly name = 'PAYMENT_AUTHORIZATION';
  private readonly logger = new Logger(PaymentConfirmationStep.name);
  private paymentServiceClient: any;

  constructor(
    private readonly commands: DeliveryCommandService,
    @Inject('PAYMENT_SERVICE') private readonly paymentClientGrpc: any,
  ) {}

  onModuleInit() {
    try {
      this.paymentServiceClient = this.paymentClientGrpc.getService('PaymentService');
    } catch (err: any) {
      this.logger.warn(`Could not bind PaymentService gRPC: ${err.message}`);
    }
  }

  async execute(context: DeliverySagaContext): Promise<DeliverySagaContext> {
    const delivery = context.delivery;
    let paymentId: string | undefined;

    if (this.paymentServiceClient) {
      const rawAmount = delivery.amount;
      const amountFloat = parseFloat(rawAmount);
      const amountMinor = Math.round(amountFloat * 100);

      if (!rawAmount || isNaN(amountFloat) || amountMinor <= 0) {
        const errorMsg = `Invalid delivery amount: "${rawAmount}". Amount must be greater than zero.`;
        this.logger.error(`[SAGA Step 1 Failed] ${errorMsg}`);
        await this.commands.updatePaymentStatus(delivery.id, PaymentStatus.FAILED);
        await this.commands.cancel(delivery.id, 'system', errorMsg);
        throw new Error(errorMsg);
      }

      try {
        this.logger.log(
          `[SAGA Step 1: Hold/Authorize] Invoking PaymentService.CreatePayment for delivery ${delivery.id} (${amountMinor} minor units)`,
        );

        const res: any = await lastValueFrom(
          this.paymentServiceClient.CreatePayment({
            deliveryId: delivery.id,
            delivery_id: delivery.id,
            userId: delivery.customerId,
            user_id: delivery.customerId,
            amountMinor: amountMinor,
            amount_minor: amountMinor,
            currency: delivery.currency || 'USD',
            idempotencyKey: `delivery-${delivery.id}-auth`,
            idempotency_key: `delivery-${delivery.id}-auth`,
          }),
        );

        paymentId =
          res?.paymentId?.value ||
          res?.payment_id?.value ||
          res?.paymentId ||
          res?.payment_id;
        const status = (res?.status || '').toUpperCase();
        this.logger.log(
          `[SAGA Step 1] Payment hold/authorization response: ID=${paymentId}, Status=${status}`,
        );

        const checkoutUrl = res?.paymentLink || res?.payment_link || res?.checkoutUrl || res?.checkout_url;
        delivery.checkoutUrl = checkoutUrl;

        if (checkoutUrl || status === 'PENDING' || status === 'PROCESSING' || status === 'REQUIRES_PAYMENT' || status === 'REQUIRES_ACTION') {
          // Async checkout required - customer needs to pay via checkoutUrl before driver dispatch
          this.logger.log(`[SAGA Step 1] Payment requires customer completion via Checkout URL: ${checkoutUrl}`);
          await this.commands.transition(
            delivery.id,
            DeliveryStatus.PENDING_PAYMENT,
            undefined,
            'Awaiting customer checkout payment',
          );
          const updated = await this.commands.updatePaymentStatus(delivery.id, PaymentStatus.PENDING);
          updated.checkoutUrl = checkoutUrl;
          return {
            delivery: updated,
            paymentId,
          };
        } else if (status === 'AUTHORIZED' || status === 'SUCCEEDED') {
          // Funds held immediately via pre-saved payment method
          await this.commands.transition(
            delivery.id,
            DeliveryStatus.PAYMENT_CONFIRMED,
            undefined,
            'Payment authorized & held in escrow',
          );
          const updated = await this.commands.updatePaymentStatus(delivery.id, PaymentStatus.AUTHORIZED);
          updated.checkoutUrl = checkoutUrl;
          return {
            delivery: updated,
            paymentId,
          };
        } else {
          const errMsg = `Payment authorization incomplete. Current status: ${status || 'FAILED'}.`;
          this.logger.error(`[SAGA Step 1 Failed] ${errMsg}`);
          await this.commands.updatePaymentStatus(delivery.id, PaymentStatus.FAILED);
          await this.commands.cancel(delivery.id, 'system', errMsg);
          throw new Error(errMsg);
        }
      } catch (err: any) {
        this.logger.error(
          `[SAGA Step 1 Failed] Payment authorization failed for delivery ${delivery.id}: ${err.message}`,
        );
        // Mark payment status as failed and cancel delivery
        await this.commands.updatePaymentStatus(delivery.id, PaymentStatus.FAILED);
        await this.commands.cancel(delivery.id, 'system', `Payment authorization failed: ${err.message}`);
        throw err;
      }
    }

    return {
      delivery,
      paymentId,
    };
  }

  async compensate(context: DeliverySagaContext): Promise<void> {
    this.logger.warn(
      `[SAGA Compensation] Cancelling payment hold / authorization for delivery ${context.delivery.id}`,
    );

    if (this.paymentServiceClient && context.paymentId) {
      try {
        await lastValueFrom(
          this.paymentServiceClient.CancelAuthorization({
            payment_id: context.paymentId,
            idempotency_key: `delivery-${context.delivery.id}-void`,
          }),
        );
        this.logger.log(`[SAGA Compensation] Payment ${context.paymentId} authorization cancelled.`);
      } catch (err: any) {
        this.logger.error(
          `[SAGA Compensation] Failed to cancel payment authorization: ${err.message}`,
        );
      }
    }

    await this.commands.cancel(
      context.delivery.id,
      'saga',
      'Saga compensation: delivery process cancelled',
    );
    await this.commands.updatePaymentStatus(context.delivery.id, PaymentStatus.CANCELLED);
  }
}
