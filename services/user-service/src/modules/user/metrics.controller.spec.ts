import { MetricsController } from './metrics.controller';
import { MetricsService } from '@delivery-micro/shard';

describe('MetricsController', () => {
  let controller: MetricsController;
  let metricsService: jest.Mocked<MetricsService>;

  function mockResponse() {
    const res: any = {
      set: jest.fn(),
      end: jest.fn(),
    };
    return res;
  }

  const exposition =
    '# HELP app_requests_total Total requests\n# TYPE app_requests_total counter\n';

  beforeEach(() => {
    metricsService = {
      getContentType: jest.fn().mockReturnValue('text/plain; version=0.0.4'),
      getMetrics: jest.fn().mockResolvedValue(exposition),
    } as unknown as jest.Mocked<MetricsService>;

    controller = new MetricsController(metricsService);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('GET /user/metrics', () => {
    it('sets the prometheus content type before writing the payload', async () => {
      const res = mockResponse();

      await controller.getMetrics(res);

      expect(metricsService.getContentType).toHaveBeenCalledTimes(1);
      expect(res.set).toHaveBeenCalledWith(
        'Content-Type',
        'text/plain; version=0.0.4',
      );
      expect(metricsService.getMetrics).toHaveBeenCalledTimes(1);
      expect(res.end).toHaveBeenCalledWith(exposition);
    });

    it('awaits the registry before ending the response', async () => {
      let resolveMetrics: (value: string) => void = () => undefined;
      metricsService.getMetrics.mockReturnValue(
        new Promise<string>((resolve) => {
          resolveMetrics = resolve;
        }),
      );
      const res = mockResponse();

      const pending = controller.getMetrics(res);
      expect(res.end).not.toHaveBeenCalled();

      resolveMetrics('late payload');
      await pending;

      expect(res.end).toHaveBeenCalledWith('late payload');
    });
  });
});
