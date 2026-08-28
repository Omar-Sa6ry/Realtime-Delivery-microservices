import { BadRequestException, Injectable } from '@nestjs/common';
import { DeliveryStatus } from '../enums/delivery-status.enum';

@Injectable()
export class DeliveryStateMachine {
  private readonly transitions: Record<
    DeliveryStatus,
    readonly DeliveryStatus[]
  > = {
    [DeliveryStatus.CREATED]: [
      DeliveryStatus.PENDING_PAYMENT,
      DeliveryStatus.CANCELLED,
    ],
    [DeliveryStatus.PENDING_PAYMENT]: [
      DeliveryStatus.PAYMENT_CONFIRMED,
      DeliveryStatus.CANCELLED,
      DeliveryStatus.FAILED,
    ],
    [DeliveryStatus.PAYMENT_CONFIRMED]: [
      DeliveryStatus.DRIVER_ASSIGNED,
      DeliveryStatus.CANCELLED,
    ],
    [DeliveryStatus.DRIVER_ASSIGNED]: [
      DeliveryStatus.DRIVER_ACCEPTED,
      DeliveryStatus.CANCELLED,
    ],
    [DeliveryStatus.DRIVER_ACCEPTED]: [
      DeliveryStatus.PICKED_UP,
      DeliveryStatus.CANCELLED,
    ],
    [DeliveryStatus.PICKED_UP]: [
      DeliveryStatus.IN_TRANSIT,
      DeliveryStatus.CANCELLED,
    ],
    [DeliveryStatus.IN_TRANSIT]: [
      DeliveryStatus.DELIVERED,
      DeliveryStatus.FAILED,
    ],
    [DeliveryStatus.DELIVERED]: [DeliveryStatus.COMPLETED],
    [DeliveryStatus.COMPLETED]: [],
    [DeliveryStatus.CANCELLED]: [],
    [DeliveryStatus.FAILED]: [],
  };
  
  canTransition(from: DeliveryStatus, to: DeliveryStatus): boolean {
    return this.transitions[from]?.includes(to) ?? false;
  }

  assertTransition(from: DeliveryStatus, to: DeliveryStatus): void {
    if (!this.canTransition(from, to))
      throw new BadRequestException(
        `Invalid delivery transition: ${from} -> ${to}`,
      );
  }

  nextStates(status: DeliveryStatus): readonly DeliveryStatus[] {
    return this.transitions[status] ?? [];
  }
}
