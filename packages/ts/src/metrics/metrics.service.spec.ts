import * as client from 'prom-client';
import { MetricsService } from './metrics.service';

describe('MetricsService', () => {
  let service: MetricsService;

  beforeEach(() => {
    client.register.clear();
    (MetricsService as any).defaultMetricsRegistered = false;
    service = new MetricsService();
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
    service.onModuleInit();
    expect(service.requestCounter).toBeDefined();
  });

  it('registers default metrics on first initialization', () => {
    service.onModuleInit();

    expect(
      client.register.getSingleMetric('process_cpu_user_seconds_total'),
    ).toBeDefined();
  });

  it('registers the application metrics with the expected labels', () => {
    service.onModuleInit();

    const requestCounter = client.register.getSingleMetric(
      'app_requests_total',
    ) as client.Counter<string>;
    const duration = client.register.getSingleMetric(
      'app_request_duration_seconds',
    ) as client.Histogram<string>;
    const errorCounter = client.register.getSingleMetric(
      'app_errors_total',
    ) as client.Counter<string>;

    expect(requestCounter).toBeDefined();
    expect(duration).toBeDefined();
    expect(errorCounter).toBeDefined();
    expect(service.requestCounter).toBe(requestCounter);
    expect(service.requestDuration).toBe(duration);
    expect(service.errorCounter).toBe(errorCounter);
    expect(requestCounter.help).toContain('Total number of requests');
    expect(errorCounter.help).toBe('Total number of application errors');
  });

  it('reuses already registered metrics on re-initialization', () => {
    service.onModuleInit();
    const counterBefore = service.requestCounter;
    const durationBefore = service.requestDuration;
    const errorsBefore = service.errorCounter;

    service.onModuleInit();

    expect(service.requestCounter).toBe(counterBefore);
    expect(service.requestDuration).toBe(durationBefore);
    expect(service.errorCounter).toBe(errorsBefore);
    expect(
      client.register.getSingleMetric('app_requests_total'),
    ).toBe(counterBefore);
  });

  it('does not re-register default metrics when the flag is already set', () => {
    service.onModuleInit();
    client.register.clear();

    const second = new MetricsService();
    second.onModuleInit();

    expect(
      client.register.getSingleMetric('process_cpu_user_seconds_total'),
    ).toBeUndefined();
    expect(
      client.register.getSingleMetric('app_requests_total'),
    ).toBeDefined();
    expect(second.requestCounter).toBeDefined();
  });

  it('increments the request counter with labels', async () => {
    service.onModuleInit();
    const labels = {
      protocol: 'HTTP',
      method: 'GET',
      path: '/orders',
      statusCode: '200',
    };

    service.requestCounter.inc(labels);
    service.requestCounter.inc(labels);

    const values = (await service.requestCounter.get()).values;
    expect(values).toHaveLength(1);
    expect(values[0].labels).toEqual(labels);
    expect(values[0].value).toBe(2);
  });

  it('observes request durations in the histogram', async () => {
    service.onModuleInit();
    const labels = {
      protocol: 'gRPC',
      method: 'RPC',
      path: 'CreateOrder',
      statusCode: '200',
    };

    service.requestDuration.observe(labels, 0.25);

    const values = (await service.requestDuration.get()).values;
    const count = values.find((v) => v.metricName.endsWith('_count'));
    const sum = values.find((v) => v.metricName.endsWith('_sum'));
    expect(count?.value).toBe(1);
    expect(sum?.value).toBeCloseTo(0.25, 10);
  });

  it('increments the error counter with context and code labels', async () => {
    service.onModuleInit();
    service.errorCounter.inc({
      context: 'HTTP:/orders',
      errorCode: 'NotFoundError',
    });

    const values = (await service.errorCounter.get()).values;
    expect(values).toHaveLength(1);
    expect(values[0].labels).toEqual({
      context: 'HTTP:/orders',
      errorCode: 'NotFoundError',
    });
    expect(values[0].value).toBe(1);
  });

  it('exposes the exposition text through getMetrics', async () => {
    service.onModuleInit();
    service.requestCounter.inc({
      protocol: 'HTTP',
      method: 'GET',
      path: '/',
      statusCode: '200',
    });

    const text = await service.getMetrics();

    expect(typeof text).toBe('string');
    expect(text).toContain('app_requests_total');
    expect(text).toContain('app_request_duration_seconds');
    expect(text).toContain('app_errors_total');
  });

  it('exposes the registry content type', () => {
    service.onModuleInit();

    expect(service.getContentType()).toBe(client.register.contentType);
    expect(service.getContentType()).toContain('text/plain');
  });
});
