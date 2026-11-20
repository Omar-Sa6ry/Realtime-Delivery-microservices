import { MetricsController } from './metrics.controller';
import { MetricsService } from '@delivery-micro/shard';

describe('MetricsController', () => {
  let controller: MetricsController;
  let metricsService: { getContentType: jest.Mock; getMetrics: jest.Mock };
  let res: { set: jest.Mock; end: jest.Mock };

  beforeEach(() => {
    metricsService = {
      getContentType: jest.fn().mockReturnValue('text/plain; version=0.0.4'),
      getMetrics: jest.fn().mockResolvedValue('# HELP http_requests_total 1'),
    };
    controller = new MetricsController(
      metricsService as unknown as MetricsService,
    );
    res = { set: jest.fn(), end: jest.fn() };
  });

  it('sets the Prometheus content type on the response', async () => {
    await controller.getMetrics(res as any);

    expect(res.set).toHaveBeenCalledWith(
      'Content-Type',
      'text/plain; version=0.0.4',
    );
  });

  it('writes the metrics payload to the response', async () => {
    await controller.getMetrics(res as any);

    expect(res.end).toHaveBeenCalledWith('# HELP http_requests_total 1');
  });

  it('awaits the metrics before ending the response', async () => {
    await controller.getMetrics(res as any);

    expect(metricsService.getMetrics).toHaveBeenCalledTimes(1);
  });

  it('propagates a metrics fetch error', async () => {
    metricsService.getMetrics.mockRejectedValue(new Error('boom'));

    await expect(controller.getMetrics(res as any)).rejects.toThrow('boom');
  });
});
