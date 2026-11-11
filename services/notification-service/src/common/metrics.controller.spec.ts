import { MetricsController } from './metrics.controller';
import { MetricsService } from '@delivery-micro/shard';

describe('MetricsController', () => {
  let controller: MetricsController;
  let metricsService: {
    getMetrics: jest.Mock;
    getContentType: jest.Mock;
  };

  beforeEach(() => {
    metricsService = {
      getMetrics: jest.fn().mockResolvedValue('# HELP http_requests_total 1'),
      getContentType: jest
        .fn()
        .mockReturnValue('text/plain; version=0.0.4; charset=utf-8'),
    };
    controller = new MetricsController(
      metricsService as unknown as MetricsService,
    );
  });

  it('streams prometheus metrics with the correct content type', async () => {
    const res: any = {
      setHeader: jest.fn(),
      end: jest.fn(),
    };

    await controller.getMetrics(res);

    expect(res.setHeader).toHaveBeenCalledWith(
      'Content-Type',
      'text/plain; version=0.0.4; charset=utf-8',
    );
    expect(res.end).toHaveBeenCalledWith('# HELP http_requests_total 1');
  });
});
