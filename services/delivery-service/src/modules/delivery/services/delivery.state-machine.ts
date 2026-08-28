import { BadRequestException, Injectable } from '@nestjs/common';
import { I18nService } from 'nestjs-i18n';
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
  constructor(private readonly i18n: I18nService) {}
  canTransition(from: DeliveryStatus, to: DeliveryStatus): boolean {
    return this.transitions[from]?.includes(to) ?? false;
  }
  async assertTransition(
    from: DeliveryStatus,
    to: DeliveryStatus,
  ): Promise<void> {
    if (!this.canTransition(from, to)) {
      const message = await this.i18n.t('delivery.invalidTransition', {
        args: { from, to },
      });
      throw new BadRequestException(message);
    }
  }
  nextStates(status: DeliveryStatus): readonly DeliveryStatus[] {
    return this.transitions[status] ?? [];
  }
}
