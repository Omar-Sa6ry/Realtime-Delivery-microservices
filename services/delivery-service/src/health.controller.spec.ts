import { Test, TestingModule } from '@nestjs/testing';
import { Response } from 'express';
import { HealthController } from './health.controller';
import { DeliveryMetricsService } from './common/metrics/delivery-metrics.service';
import { createMockDeliveryMetricsService } from './test-utils';

describe('HealthController', () => {
  let controller: HealthController;
  let metricsService: ReturnType<typeof createMockDeliveryMetricsService>;
  let mockResponse: Partial<Response>;

  beforeEach(async () => {
    metricsService = createMockDeliveryMetricsService();
    mockResponse = {
      type: jest.fn().mockReturnThis(),
      send: jest.fn().mockReturnThis(),
    };

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        HealthController,
        { provide: DeliveryMetricsService, useValue: metricsService },
      ],
    }).compile();

    controller = module.get<HealthController>(HealthController);
  });

  it('should be defined', () => {
    expect(controller).toBeDefined();
  });

  describe('getHealth', () => {
    it('should return health status', () => {
      const result = controller.getHealth();

      expect(result).toEqual({
        status: 'UP',
        service: 'delivery-service',
        timestamp: expect.any(String),
      });
      expect(new Date(result.timestamp)).toBeInstanceOf(Date);
    });
  });

  describe('getLiveness', () => {
    it('should return liveness status', () => {
      const result = controller.getLiveness();

      expect(result).toEqual({ status: 'UP' });
    });
  });

  describe('getReadiness', () => {
    it('should return readiness status', () => {
      const result = controller.getReadiness();

      expect(result).toEqual({ status: 'UP' });
    });
  });

  describe('getMetrics', () => {
    it('should return metrics in prometheus format', async () => {
      metricsService.getContentType.mockReturnValue('text/plain; version=0.0.4; charset=utf-8');
      metricsService.getMetrics.mockResolvedValue('# HELP test_metric Test metric\n# TYPE test_metric gauge\ntest_metric 1\n');

      await controller.getMetrics(mockResponse as Response);

      expect(mockResponse.type).toHaveBeenCalledWith('text/plain; version=0.0.4; charset=utf-8');
      expect(mockResponse.send).toHaveBeenCalledWith('# HELP test_metric Test metric\n# TYPE test_metric gauge\ntest_metric 1\n');
      expect(metricsService.getContentType).toHaveBeenCalled();
      expect(metricsService.getMetrics).toHaveBeenCalled();
    });
  });
});