jest.mock('amqplib', () => ({ connect: jest.fn() }));

import { Logger } from '@nestjs/common';
import { CircuitBreakerState, RabbitMQService } from './rabbitmq.service';
import { RabbitMQHealthIndicator } from './rabbitmq.health';

describe('RabbitMQHealthIndicator', () => {
  let rabbitmq: {
    isConnected: jest.Mock;
    getCircuitBreakerState: jest.Mock;
    isTopologyAsserted: jest.Mock;
  };
  let indicator: RabbitMQHealthIndicator;
  let errorSpy: jest.SpyInstance;

  beforeEach(() => {
    errorSpy = jest.spyOn(Logger.prototype, 'error').mockImplementation(() => undefined);
    rabbitmq = {
      isConnected: jest.fn(),
      getCircuitBreakerState: jest.fn(),
      isTopologyAsserted: jest.fn(),
    };
    indicator = new RabbitMQHealthIndicator(rabbitmq as unknown as RabbitMQService);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('reports UP when connected with a closed breaker and asserted topology', async () => {
    rabbitmq.isConnected.mockReturnValue(true);
    rabbitmq.getCircuitBreakerState.mockReturnValue(CircuitBreakerState.CLOSED);
    rabbitmq.isTopologyAsserted.mockReturnValue(true);

    await expect(indicator.isHealthy()).resolves.toEqual({
      rabbitmq: {
        status: 'up',
        connected: true,
        circuitBreaker: 'CLOSED',
        topologyAsserted: true,
      },
    });
  });

  it('reports UP while the breaker is half-open with pending topology', async () => {
    rabbitmq.isConnected.mockReturnValue(true);
    rabbitmq.getCircuitBreakerState.mockReturnValue(CircuitBreakerState.HALF_OPEN);
    rabbitmq.isTopologyAsserted.mockReturnValue(false);

    const result = await indicator.isHealthy('broker');

    expect(result).toEqual({
      broker: {
        status: 'up',
        connected: true,
        circuitBreaker: 'HALF_OPEN',
        topologyAsserted: false,
      },
    });
  });

  it('reports DOWN when the service is not connected', async () => {
    rabbitmq.isConnected.mockReturnValue(false);
    rabbitmq.getCircuitBreakerState.mockReturnValue(CircuitBreakerState.CLOSED);

    await expect(indicator.isHealthy()).resolves.toEqual({
      rabbitmq: {
        status: 'down',
        connected: false,
        circuitBreaker: 'CLOSED',
      },
    });
    expect(rabbitmq.isTopologyAsserted).not.toHaveBeenCalled();
  });

  it('reports DOWN when the circuit breaker is open', async () => {
    rabbitmq.isConnected.mockReturnValue(true);
    rabbitmq.getCircuitBreakerState.mockReturnValue(CircuitBreakerState.OPEN);

    await expect(indicator.isHealthy('broker')).resolves.toEqual({
      broker: {
        status: 'down',
        connected: true,
        circuitBreaker: 'OPEN',
      },
    });
    expect(rabbitmq.isTopologyAsserted).not.toHaveBeenCalled();
  });

  it('reports DOWN with error details when the check throws', async () => {
    rabbitmq.isConnected.mockImplementation(() => {
      throw new Error('broker exploded');
    });

    await expect(indicator.isHealthy()).resolves.toEqual({
      rabbitmq: {
        status: 'down',
        message: 'broker exploded',
      },
    });
    expect(errorSpy).toHaveBeenCalledWith('RabbitMQ health check failed: broker exploded');
  });

  it('reports DOWN when reading the breaker state throws', async () => {
    rabbitmq.isConnected.mockReturnValue(true);
    rabbitmq.getCircuitBreakerState.mockImplementation(() => {
      throw new Error('breaker unavailable');
    });

    await expect(indicator.isHealthy('broker')).resolves.toEqual({
      broker: {
        status: 'down',
        message: 'breaker unavailable',
      },
    });
    expect(errorSpy).toHaveBeenCalledWith('RabbitMQ health check failed: breaker unavailable');
  });
});
