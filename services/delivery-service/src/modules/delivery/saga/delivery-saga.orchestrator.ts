import { Injectable, Logger } from '@nestjs/common';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliverySagaContext, DeliverySagaStep } from './saga-step';
import { PaymentConfirmationStep } from './steps/payment-confirmation.step';
import { DriverAssignmentStep } from './steps/driver-assignment.step';

@Injectable()
export class DeliverySagaOrchestrator {
  private readonly logger = new Logger(DeliverySagaOrchestrator.name);
  private readonly steps: DeliverySagaStep[];

  constructor(
    private readonly repository: DeliveryRepository,
    paymentStep: PaymentConfirmationStep,
    driverStep: DriverAssignmentStep,
  ) {
    this.steps = [paymentStep, driverStep];
  }

  async execute(deliveryId: string): Promise<Delivery> {
    const delivery = await this.repository.findById(deliveryId);
    if (!delivery) {
      throw new Error(`Delivery with ID ${deliveryId} not found for Saga execution`);
    }

    let context: DeliverySagaContext = {
      delivery,
    };
    const completed: DeliverySagaStep[] = [];

    try {
      this.logger.log(`Starting Saga execution for delivery ${deliveryId}...`);
      for (const step of this.steps) {
        this.logger.log(`Executing Saga step: ${step.name}`);
        context = await step.execute(context);
        completed.push(step);
      }
      this.logger.log(`Saga completed successfully for delivery ${deliveryId}.`);
      return context.delivery;
    } catch (error: any) {
      this.logger.error(
        `Saga failed at step for delivery ${deliveryId}: ${error.message}. Initiating compensations...`,
      );
      for (const step of completed.reverse()) {
        try {
          this.logger.warn(`Compensating Saga step: ${step.name}`);
          await step.compensate(context);
        } catch (compErr: any) {
          this.logger.error(
            `Compensation failed for step ${step.name}: ${compErr.message}`,
          );
        }
      }
      throw error;
    }
  }
}
