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

    if (this.paymentServiceClient && delivery.amount) {
      try {
        const amountMinor = Math.round(parseFloat(delivery.amount) * 100);
        this.logger.log(
          `[SAGA Step 1: Hold/Authorize] Invoking PaymentService.CreatePayment for delivery ${delivery.id} (${amountMinor} minor units)`,
        );

        const res: any = await lastValueFrom(
          this.paymentServiceClient.CreatePayment({
            delivery_id: delivery.id,
            user_id: delivery.customerId,
            amount_minor: amountMinor,
            currency: delivery.currency || 'USD',
            idempotency_key: `delivery-${delivery.id}-auth`,
          }),
        );

        paymentId = res?.payment_id?.value || res?.payment_id;
        this.logger.log(
          `[SAGA Step 1] Payment hold/authorization created successfully: ID=${paymentId}, Status=${res?.status}`,
        );
      } catch (err: any) {
        this.logger.error(
          `[SAGA Step 1 Failed] Payment authorization failed for delivery ${delivery.id}: ${err.message}`,
        );
        // Mark payment status as failed and cancel delivery
        await this.commands.updatePaymentStatus(delivery.id, PaymentStatus.FAILED);
        throw err;
      }
    }

    // Advance delivery status to PAYMENT_CONFIRMED (funds authorized/held)
    const updated = await this.commands.transition(
      delivery.id,
      DeliveryStatus.PAYMENT_CONFIRMED,
      undefined,
      'Payment authorized & held in escrow',
    );
    await this.commands.updatePaymentStatus(delivery.id, PaymentStatus.AUTHORIZED);

    return {
      delivery: updated,
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
