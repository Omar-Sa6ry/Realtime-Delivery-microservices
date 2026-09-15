import { Inject, Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { lastValueFrom } from 'rxjs';
import { DeliveryStatus } from '../../enums/delivery-status.enum';
import { DeliveryCommandService } from '../../services/delivery-command.service';
import { DeliverySagaContext, DeliverySagaStep } from '../saga-step';

@Injectable()
export class DriverAssignmentStep implements DeliverySagaStep, OnModuleInit {
  readonly name = 'DRIVER_ASSIGNMENT';
  private readonly logger = new Logger(DriverAssignmentStep.name);
  private driverServiceClient: any;

  constructor(
    private readonly commands: DeliveryCommandService,
    @Inject('DRIVER_SERVICE') private readonly driverClientGrpc: any,
  ) {}

  onModuleInit() {
    try {
      this.driverServiceClient = this.driverClientGrpc.getService('DriverService');
    } catch (err: any) {
      this.logger.warn(`Could not bind DriverService gRPC: ${err.message}`);
    }
  }

  async execute(context: DeliverySagaContext): Promise<DeliverySagaContext> {
    const delivery = context.delivery;
    this.logger.log(
      `[SAGA Step 2: Driver Search / Offer] Preparing driver assignment step for delivery ${delivery.id}`,
    );

    // If a driver was already assigned or specified
    if (delivery.driverId && this.driverServiceClient) {
      try {
        this.logger.log(
          `[SAGA Step 2] Reserving driver ${delivery.driverId} via DriverService gRPC...`,
        );
        const res: any = await lastValueFrom(
          this.driverServiceClient.ReserveDriver({
            driverId: delivery.driverId,
            deliveryId: delivery.id,
            idempotencyKey: `reserve-${delivery.id}-${delivery.driverId}`,
            correlationId: delivery.id,
          }),
        );
        this.logger.log(
          `[SAGA Step 2] Driver ${delivery.driverId} reserved: ${res?.reserved}`,
        );
      } catch (err: any) {
        this.logger.error(
          `[SAGA Step 2] Failed to reserve driver ${delivery.driverId} via gRPC: ${err.message}`,
        );
        throw err;
      }
    }

    return context;
  }

  async compensate(context: DeliverySagaContext): Promise<void> {
    const delivery = context.delivery;
    this.logger.warn(
      `[SAGA Compensation: Driver] Releasing driver reservation for delivery ${delivery.id}`,
    );

    if (this.driverServiceClient && delivery.driverId) {
      try {
        await lastValueFrom(
          this.driverServiceClient.ReleaseDriver({
            driverId: delivery.driverId,
            deliveryId: delivery.id,
            reason: 'Saga compensation',
            correlationId: delivery.id,
          }),
        );
        this.logger.log(`[SAGA Compensation] Driver ${delivery.driverId} released.`);
      } catch (err: any) {
        this.logger.error(
          `[SAGA Compensation] Failed to release driver via gRPC: ${err.message}`,
        );
      }
    }
  }
}
