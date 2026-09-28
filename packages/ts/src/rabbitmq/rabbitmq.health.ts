import { Injectable, Logger } from "@nestjs/common";
import { HealthIndicator, HealthIndicatorResult } from "@nestjs/terminus";
import { RabbitMQService } from "./rabbitmq.service";
import { CircuitBreakerState } from "./rabbitmq.service";

@Injectable()
export class RabbitMQHealthIndicator extends HealthIndicator {
  private readonly logger = new Logger(RabbitMQHealthIndicator.name);

  constructor(private readonly rabbitmq: RabbitMQService) {
    super();
  }

  async isHealthy(key = "rabbitmq"): Promise<HealthIndicatorResult> {
    try {
      const connected = this.rabbitmq.isConnected();
      const breakerState = this.rabbitmq.getCircuitBreakerState();
      const healthy = connected && breakerState !== CircuitBreakerState.OPEN;

      if (!healthy) {
        return this.getStatus(key, false, {
          connected,
          circuitBreaker: CircuitBreakerState[breakerState],
        });
      }

      const channel = await this.rabbitmq.createConsumerChannel(1);
      try {
        await channel.close();
      } catch {}

      return this.getStatus(key, true, {
        connected,
        circuitBreaker: CircuitBreakerState[breakerState],
      });
    } catch (err) {
      this.logger.error(
        `RabbitMQ health check failed: ${(err as Error).message}`,
      );
      return this.getStatus(key, false, { message: (err as Error).message });
    }
  }
}
