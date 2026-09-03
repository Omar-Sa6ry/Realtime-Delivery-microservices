import { Injectable, Optional } from '@nestjs/common';
import { randomUUID } from 'crypto';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { PaymentStatus } from '../enums/payment-status.enum';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliveryStateMachine } from './delivery.state-machine';
import { IdempotencyService } from './idempotency.service';
import { OutboxRepository } from '../outbox/outbox.repository';
import { DeliveryKafkaTopics, NatsService, RealtimeNatsSubjects } from '@delivery/common';

export interface CreateDeliveryInput {
  customerId: string;
  amount: string;
  currency?: string;
  pickupAddress: Delivery['pickupAddress'];
  dropoffAddress: Delivery['dropoffAddress'];
  idempotencyKey?: string;
}

@Injectable()
export class DeliveryCommandService {
  constructor(
    private readonly repository: DeliveryRepository,
    private readonly stateMachine: DeliveryStateMachine,
    private readonly idempotency: IdempotencyService,
    private readonly outbox: OutboxRepository,
    @Optional() private readonly nats?: NatsService,
  ) {}

  async create(input: CreateDeliveryInput): Promise<Delivery> {
    const operation = async () => {
      const delivery = await this.repository.create({
        customerId: input.customerId,
        amount: input.amount,
        currency: input.currency ?? 'USD',
        pickupAddress: input.pickupAddress,
        dropoffAddress: input.dropoffAddress,
        status: DeliveryStatus.CREATED,
        paymentStatus: PaymentStatus.PENDING,
      });

      // Write domain event to transactional outbox
      await this.outbox.save(
        this.outbox.createEvent({
          eventId: randomUUID(),
          eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
          aggregateId: delivery.id,
          payload: {
            deliveryId: delivery.id,
            customerId: delivery.customerId,
            driverId: delivery.driverId,
            status: delivery.status,
            amount: delivery.amount,
            currency: delivery.currency,
            pickup: delivery.pickupAddress,
            dropoff: delivery.dropoffAddress,
            createdAt: delivery.createdAt?.toISOString() ?? new Date().toISOString(),
          },
        }),
      );

      // Low-latency NATS notify to realtime service
      this.publishNats(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, {
        deliveryId: delivery.id,
        customerId: delivery.customerId,
        driverId: delivery.driverId,
        status: delivery.status,
        timestamp: Date.now(),
      });

      return delivery;
    };

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
    await this.stateMachine.assertTransition(delivery.status, status);
    delivery.status = status;
    if (status === DeliveryStatus.PICKED_UP) delivery.pickedUpAt = new Date();
    if (status === DeliveryStatus.COMPLETED) delivery.completedAt = new Date();
    if (status === DeliveryStatus.CANCELLED) delivery.cancelledAt = new Date();
    await this.repository.appendHistory(delivery, status, changedBy, note);
    const saved = await this.repository.save(delivery);

    // Map status to Kafka domain event
    const eventType = this.statusToKafkaTopic(status);
    if (eventType) {
      await this.outbox.save(
        this.outbox.createEvent({
          eventId: randomUUID(),
          eventType,
          aggregateId: saved.id,
          payload: {
            deliveryId: saved.id,
            customerId: saved.customerId,
            driverId: saved.driverId,
            status: saved.status,
            changedBy: changedBy ?? null,
            note: note ?? null,
            updatedAt: saved.updatedAt?.toISOString() ?? new Date().toISOString(),
          },
        }),
      );
    }

    // Fast-path NATS event to Realtime service
    this.publishNats(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, {
      deliveryId: saved.id,
      customerId: saved.customerId,
      driverId: saved.driverId,
      status: saved.status,
      timestamp: Date.now(),
    });

    return saved;
  }

  async updatePaymentStatus(id: string, paymentStatus: PaymentStatus): Promise<Delivery> {
    const delivery = await this.repository.findById(id);
    delivery.paymentStatus = paymentStatus;
    return this.repository.save(delivery);
  }

  cancel(id: string, changedBy?: string, note?: string): Promise<Delivery> {
    return this.transition(id, DeliveryStatus.CANCELLED, changedBy, note);
  }

  private statusToKafkaTopic(status: DeliveryStatus): string | null {
    switch (status) {
      case DeliveryStatus.DRIVER_ASSIGNED:
        return DeliveryKafkaTopics.DRIVER_ASSIGNED;
      case DeliveryStatus.DRIVER_ACCEPTED:
        return DeliveryKafkaTopics.DRIVER_ACCEPTED;
      case DeliveryStatus.PICKED_UP:
        return DeliveryKafkaTopics.DELIVERY_PICKED_UP;
      case DeliveryStatus.IN_TRANSIT:
        return DeliveryKafkaTopics.DELIVERY_IN_TRANSIT;
      case DeliveryStatus.DELIVERED:
      case DeliveryStatus.COMPLETED:
        return DeliveryKafkaTopics.DELIVERY_COMPLETED;
      case DeliveryStatus.CANCELLED:
        return DeliveryKafkaTopics.DELIVERY_CANCELLED;
      default:
        return null;
    }
  }

  private publishNats(subject: string, data: any): void {
    if (this.nats) {
      try {
        this.nats.emit(subject, data);
      } catch {
        /* NATS emission is best-effort; Kafka Outbox is source of truth */
      }
    }
  }
}
