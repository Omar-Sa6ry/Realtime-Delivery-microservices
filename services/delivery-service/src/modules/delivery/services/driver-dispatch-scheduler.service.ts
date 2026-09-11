import {
  Injectable,
  Logger,
  OnModuleDestroy,
  OnModuleInit,
} from '@nestjs/common';
import { DeliveryRepository } from '../repositories/delivery.repository';
import { DeliveryCommandService } from './delivery-command.service';

@Injectable()
export class DriverDispatchSchedulerService
  implements OnModuleInit, OnModuleDestroy
{
  private readonly logger = new Logger(DriverDispatchSchedulerService.name);
  private timer?: NodeJS.Timeout;
  private isProcessing = false;

  constructor(
    private readonly repository: DeliveryRepository,
    private readonly commands: DeliveryCommandService,
  ) {}

  onModuleInit(): void {
    const intervalMs = Number(
      process.env.DRIVER_DISPATCH_RETRY_INTERVAL_MS ?? 60000,
    );
    this.logger.log(
      `DriverDispatchSchedulerService initialized (interval: ${intervalMs}ms)`,
    );

    this.timer = setInterval(() => {
      void this.processUnassignedDeliveries();
    }, intervalMs);
  }

  onModuleDestroy(): void {
    if (this.timer) {
      clearInterval(this.timer);
    }
  }

  async processUnassignedDeliveries(): Promise<void> {
    if (this.isProcessing) {
      return;
    }
    this.isProcessing = true;

    try {
      const pendingDeliveries =
        await this.repository.findUnassignedPendingDeliveries();
      if (!pendingDeliveries || pendingDeliveries.length === 0) {
        return;
      }

      this.logger.log(
        `Checking ${pendingDeliveries.length} unassigned pending deliveries for driver dispatch retry...`,
      );

      for (const delivery of pendingDeliveries) {
        // Skip deliveries created less than 45 seconds ago so we don't duplicate the initial delivery.created dispatch
        const ageMs = Date.now() - new Date(delivery.createdAt).getTime();
        if (ageMs < 45000) {
          continue;
        }

        try {
          await this.commands.retryDriverDispatch(delivery);
        } catch (err: any) {
          this.logger.error(
            `Failed to retry driver dispatch for delivery ${delivery.id}: ${err?.message}`,
          );
        }
      }
    } catch (err: any) {
      this.logger.error(
        `Error during driver dispatch scheduling cycle: ${err?.message}`,
      );
    } finally {
      this.isProcessing = false;
    }
  }
}
