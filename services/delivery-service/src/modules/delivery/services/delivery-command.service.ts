import { BadRequestException, Inject, Injectable, Logger, OnModuleInit, Optional } from '@nestjs/common';
import { randomUUID } from 'crypto';
import { lastValueFrom } from 'rxjs';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { PaymentStatus } from '../enums/payment-status.enum';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliveryStateMachine } from './delivery.state-machine';
import { IdempotencyService } from './idempotency.service';
import { OutboxRepository } from '../outbox/outbox.repository';
import { DeliveryKafkaTopics, NatsService, NotificationNatsSubjects, RealtimeNatsSubjects } from '@delivery/common';

export interface CreateDeliveryInput {
  customerId: string;
  amount: string;
  currency?: string;
  pickupAddress: Delivery['pickupAddress'];
  dropoffAddress: Delivery['dropoffAddress'];
  idempotencyKey?: string;
}

@Injectable()
export class DeliveryCommandService implements OnModuleInit {
  private readonly logger = new Logger(DeliveryCommandService.name);
  private userServiceClient: any;

  constructor(
    private readonly repository: DeliveryRepository,
    private readonly stateMachine: DeliveryStateMachine,
    private readonly idempotency: IdempotencyService,
    private readonly outbox: OutboxRepository,
    @Inject('USER_SERVICE') private readonly userServiceClientGrpc: any,
    @Optional() private readonly nats?: NatsService,
  ) {}

  onModuleInit() {
    this.userServiceClient = this.userServiceClientGrpc.getService('UserService');
  }

  async create(input: CreateDeliveryInput): Promise<Delivery> {
    // Validate customer existence via gRPC
    if (this.userServiceClient) {
      try {
        const user = await lastValueFrom(this.userServiceClient.GetUser({ id: input.customerId }));
        if (!user || !(user as any).id) {
          throw new BadRequestException(`Customer with ID ${input.customerId} does not exist`);
        }
      } catch (err: any) {
        if (err instanceof BadRequestException) {
          throw err;
        }
        if (err?.code === 5 || err?.details?.includes('not found')) {
          throw new BadRequestException(`Customer with ID ${input.customerId} does not exist`);
        }
        this.logger.error(`Failed to validate customer via gRPC: ${err.message}`);
        throw new BadRequestException('Could not validate customer ID');
      }
    }

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
    if (delivery.status === status) {
      return delivery;
    }
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

  async assignDriver(id: string, driverId: string): Promise<Delivery> {
    const delivery = await this.repository.findById(id);
    delivery.driverId = driverId;
    await this.repository.save(delivery);
    return this.transition(id, DeliveryStatus.DRIVER_ASSIGNED, driverId, `Driver ${driverId} assigned`);
  }

  async acceptDriver(id: string, driverId: string): Promise<Delivery> {
    const delivery = await this.repository.findById(id);
    if (!delivery.driverId) {
      delivery.driverId = driverId;
      await this.repository.save(delivery);
    }
    if (delivery.status === DeliveryStatus.PAYMENT_CONFIRMED || delivery.status === DeliveryStatus.CREATED) {
      await this.transition(id, DeliveryStatus.DRIVER_ASSIGNED, driverId, `Driver ${driverId} assigned`);
    }
    const updated = await this.transition(id, DeliveryStatus.DRIVER_ACCEPTED, driverId, `Driver ${driverId} accepted`);

    // Notify customer via NATS notification channel
    this.publishNats(`${NotificationNatsSubjects.NOTIFICATION_USER}.${updated.customerId}`, {
      type: 'DRIVER_ACCEPTED',
      title: 'Driver Found!',
      body: `A driver has accepted your delivery request #${updated.id}.`,
      data: {
        deliveryId: updated.id,
        driverId,
        status: updated.status,
      },
    });

    // Notify customer via Realtime driver assignment channel
    this.publishNats(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED, {
      deliveryId: updated.id,
      driverId,
      status: 'ACCEPTED',
      timestamp: Date.now(),
    });

    return updated;
  }

  async retryDriverDispatch(delivery: Delivery): Promise<void> {
    if (!delivery || delivery.status !== DeliveryStatus.CREATED || delivery.driverId) {
      return;
    }

    const elapsedMs = Date.now() - new Date(delivery.createdAt).getTime();
    const tenMinutesMs = 10 * 60 * 1000;

    if (elapsedMs >= tenMinutesMs) {
      await this.handleDriverSearchTimeout(delivery.id);
      return;
    }

    this.logger.log(`Periodic retry: searching for available driver for delivery ${delivery.id}...`);
    await this.outbox.save(
      this.outbox.createEvent({
        eventId: randomUUID(),
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
        aggregateId: delivery.id,
        payload: {
          deliveryId: delivery.id,
          customerId: delivery.customerId,
          driverId: null,
          status: delivery.status,
          amount: delivery.amount,
          currency: delivery.currency,
          pickup: delivery.pickupAddress,
          dropoff: delivery.dropoffAddress,
          createdAt: delivery.createdAt?.toISOString() ?? new Date().toISOString(),
        },
      }),
    );
  }

  async handleDriverRejectedOrExpired(id: string, reason: string): Promise<void> {
    const delivery = await this.repository.findById(id);
    if (!delivery || delivery.status === DeliveryStatus.DRIVER_ACCEPTED || delivery.status === DeliveryStatus.CANCELLED || delivery.status === DeliveryStatus.FAILED || delivery.status === DeliveryStatus.COMPLETED) {
      return;
    }

    const elapsedMs = Date.now() - new Date(delivery.createdAt).getTime();
    const tenMinutesMs = 10 * 60 * 1000;

    if (elapsedMs >= tenMinutesMs) {
      await this.handleDriverSearchTimeout(id);
      return;
    }

    this.logger.log(`Driver rejected/expired for delivery ${id} (${reason}). Re-triggering driver dispatch...`);
    // Re-publish DELIVERY_CREATED event to outbox to find next available driver
    await this.outbox.save(
      this.outbox.createEvent({
        eventId: randomUUID(),
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
        aggregateId: delivery.id,
        payload: {
          deliveryId: delivery.id,
          customerId: delivery.customerId,
          driverId: null,
          status: delivery.status,
          amount: delivery.amount,
          currency: delivery.currency,
          pickup: delivery.pickupAddress,
          dropoff: delivery.dropoffAddress,
          createdAt: delivery.createdAt?.toISOString() ?? new Date().toISOString(),
        },
      }),
    );
  }

  async handleDriverSearchTimeout(id: string): Promise<void> {
    const delivery = await this.repository.findById(id);
    if (!delivery || delivery.status === DeliveryStatus.DRIVER_ACCEPTED || delivery.status === DeliveryStatus.CANCELLED || delivery.status === DeliveryStatus.FAILED || delivery.status === DeliveryStatus.COMPLETED) {
      return;
    }


    this.logger.warn(`No driver found within 10 minutes for delivery ${id}. Cancelling and notifying customer...`);
    const cancelled = await this.cancel(id, 'system', 'No driver found within 10 minutes');

    // Notify customer
    this.publishNats(`${NotificationNatsSubjects.NOTIFICATION_USER}.${cancelled.customerId}`, {
      type: 'DELIVERY_CANCELLED',
      title: 'No Driver Found',
      body: `We were unable to find an available driver for your delivery request #${cancelled.id} within 10 minutes. The request has been cancelled.`,
      data: {
        deliveryId: cancelled.id,
        status: cancelled.status,
        reason: 'NO_DRIVER_FOUND_TIMEOUT',
      },
    });
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
