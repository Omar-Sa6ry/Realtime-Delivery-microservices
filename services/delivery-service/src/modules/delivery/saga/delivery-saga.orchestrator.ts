import { Injectable } from '@nestjs/common';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliverySagaContext, DeliverySagaStep } from './saga-step';

@Injectable()
export class DeliverySagaOrchestrator {
  constructor(
    private readonly repository: DeliveryRepository,
    private readonly steps: DeliverySagaStep[],
  ) {}
  
  async execute(deliveryId: string): Promise<Delivery> {
    let context: DeliverySagaContext = {
      delivery: await this.repository.findById(deliveryId),
    };
    const completed: DeliverySagaStep[] = [];
    try {
      for (const step of this.steps) {
        context = await step.execute(context);
        completed.push(step);
      }
      return context.delivery;
    } catch (error) {
      for (const step of completed.reverse()) {
        try {
          await step.compensate(context);
        } catch {
          /* compensation is best effort */
        }
      }
      throw error;
    }
  }
}
