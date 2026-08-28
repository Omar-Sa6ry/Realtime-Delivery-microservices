import { Injectable } from '@nestjs/common';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliveryStateMachine } from './delivery.state-machine';
import { DeliveryStatus } from '../enums/delivery-status.enum';

@Injectable()
export class DeliveryQueryService {
  constructor(
    private readonly repository: DeliveryRepository,
    private readonly stateMachine: DeliveryStateMachine,
  ) {}

  getById(id: string): Promise<Delivery> {
    return this.repository.findById(id);
  }

  listByCustomer(
    customerId: string,
    page = 1,
    pageSize = 50,
  ): Promise<[Delivery[], number]> {
    const safePage = Math.max(1, page);
    const safeSize = Math.min(100, Math.max(1, pageSize));
    return this.repository.findByCustomerId(
      customerId,
      (safePage - 1) * safeSize,
      safeSize,
    );
  }
  
  nextStatuses(delivery: Delivery): DeliveryStatus[] {
    return [...this.stateMachine.nextStates(delivery.status)];
  }
}
