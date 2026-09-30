import { Injectable, Logger } from '@nestjs/common';
import { HealthIndicator, HealthIndicatorResult } from '@nestjs/terminus';
import { RabbitMQService } from './rabbitmq.service';
import { CircuitBreakerState } from './rabbitmq.service';

@Injectable()
export class RabbitMQHealthIndicator extends HealthIndicator {
  private readonly logger = new Logger(RabbitMQHealthIndicator.name);

  constructor(private readonly rabbitmq: RabbitMQService) {
    super();
  }

  async isHealthy(key = 'rabbitmq'): Promise<HealthIndicatorResult> {
    try {
      const connected = this.rabbitmq.isConnected();
      const breakerState = this.rabbitmq.getCircuitBreakerState();
      const breakerName = CircuitBreakerState[breakerState];

      if (!connected || breakerState === CircuitBreakerState.OPEN) {
        return this.getStatus(key, false, {
          connected,
          circuitBreaker: breakerName,
        });
      }

      const topologyReady = this.rabbitmq.isTopologyAsserted();

      return this.getStatus(key, true, {
        connected,
        circuitBreaker: breakerName,
        topologyAsserted: topologyReady,
      });
    } catch (err) {
      this.logger.error(
        `RabbitMQ health check failed: ${(err as Error).message}`,
      );
      return this.getStatus(key, false, { message: (err as Error).message });
    }
  }
}
