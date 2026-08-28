import { Injectable } from '@nestjs/common';
import { DeliveryStatus } from '../../enums/delivery-status.enum';
import { DeliveryCommandService } from '../../services/delivery-command.service';
import { DeliverySagaContext, DeliverySagaStep } from '../saga-step';

@Injectable()
export class DriverAssignmentStep implements DeliverySagaStep {
  readonly name = 'DRIVER_ASSIGNMENT';
  constructor(private readonly commands: DeliveryCommandService) {}

  async execute(context: DeliverySagaContext): Promise<DeliverySagaContext> {
    return {
      delivery: await this.commands.transition(
        context.delivery.id,
        DeliveryStatus.DRIVER_ASSIGNED,
      ),
    };
  }

  async compensate(context: DeliverySagaContext): Promise<void> {
    await this.commands.cancel(
      context.delivery.id,
      undefined,
      'Saga compensation',
    );
  }
}
