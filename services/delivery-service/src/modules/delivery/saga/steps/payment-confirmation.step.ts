import { Inject, Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { lastValueFrom } from 'rxjs';
import { DeliveryStatus } from '../../enums/delivery-status.enum';
import { PaymentStatus } from '../../enums/payment-status.enum';
import { DeliveryCommandService } from '../../services/delivery-command.service';
import { DeliverySagaContext, DeliverySagaStep } from '../saga-step';

@Injectable()
export class PaymentConfirmationStep implements DeliverySagaStep, OnModuleInit {
  readonly name = 'PAYMENT_CONFIRMATION';
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
    if (this.paymentServiceClient && delivery.amount) {
      try {
        const amountMinor = Math.round(parseFloat(delivery.amount) * 100);
        this.logger.log(`Invoking PaymentService.CreatePayment for delivery ${delivery.id} (${amountMinor} minor units)`);
        const res: any = await lastValueFrom(
          this.paymentServiceClient.CreatePayment({
            delivery_id: delivery.id,
            user_id: delivery.customerId,
            amount_minor: amountMinor,
            currency: delivery.currency || 'USD',
            idempotency_key: `delivery-${delivery.id}-auth`,
          }),
        );
        this.logger.log(`Payment created: ID=${res?.payment_id?.value || res?.payment_id} Status=${res?.status}`);
      } catch (err: any) {
        this.logger.error(`PaymentService gRPC call failed for delivery ${delivery.id}: ${err.message}`);
        throw err;
      }
    }

    const updated = await this.commands.transition(
      delivery.id,
      DeliveryStatus.PAYMENT_CONFIRMED,
    );
    await this.commands.updatePaymentStatus(delivery.id, PaymentStatus.COMPLETED);

    return {
      delivery: updated,
    };
  }
  
  async compensate(context: DeliverySagaContext): Promise<void> {
    await this.commands.cancel(
      context.delivery.id,
      undefined,
      'Saga compensation: payment failed',
    );
  }
}
