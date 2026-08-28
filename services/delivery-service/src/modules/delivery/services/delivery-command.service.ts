import { Injectable } from '@nestjs/common';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { PaymentStatus } from '../enums/payment-status.enum';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliveryStateMachine } from './delivery.state-machine';
import { IdempotencyService } from './idempotency.service';

export interface CreateDeliveryInput {
  customerId: string;
  amount: string;
  currency?: string;
  pickupAddress: Record<string, unknown>;
  dropoffAddress: Record<string, unknown>;
  idempotencyKey?: string;
}

@Injectable()
export class DeliveryCommandService {
  constructor(
    private readonly repository: DeliveryRepository,
    private readonly stateMachine: DeliveryStateMachine,
    private readonly idempotency: IdempotencyService,
  ) {}
  
  create(input: CreateDeliveryInput): Promise<Delivery> {
    const operation = () =>
      this.repository.create({
        customerId: input.customerId,
        amount: input.amount,
        currency: input.currency ?? 'USD',
        pickupAddress:
          input.pickupAddress as unknown as Delivery['pickupAddress'],
        dropoffAddress:
          input.dropoffAddress as unknown as Delivery['dropoffAddress'],
        status: DeliveryStatus.CREATED,
        paymentStatus: PaymentStatus.PENDING,
      });
    return input.idempotencyKey
      ? this.idempotency.execute(input.idempotencyKey, operation)
      : operation();
  }

  async transition(
    id: string,
    status: DeliveryStatus,
    changedBy?: string,
    note?: string,
  ): Promise<Delivery> {
    const delivery = await this.repository.findById(id);
    this.stateMachine.assertTransition(delivery.status, status);
    delivery.status = status;
    if (status === DeliveryStatus.PICKED_UP) delivery.pickedUpAt = new Date();
    if (status === DeliveryStatus.COMPLETED) delivery.completedAt = new Date();
    if (status === DeliveryStatus.CANCELLED) delivery.cancelledAt = new Date();
    await this.repository.appendHistory(delivery, status, changedBy, note);
    return this.repository.save(delivery);
  }
  cancel(id: string, changedBy?: string, note?: string): Promise<Delivery> {
    return this.transition(id, DeliveryStatus.CANCELLED, changedBy, note);
  }
}
