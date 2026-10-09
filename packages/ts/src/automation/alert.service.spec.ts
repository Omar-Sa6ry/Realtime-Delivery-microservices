import { Logger } from '@nestjs/common';
import { AlertService } from './alert.service';

describe('AlertService', () => {
  let service: AlertService;
  let fetchSpy: jest.SpyInstance;
  let warnSpy: jest.SpyInstance;
  let logSpy: jest.SpyInstance;
  let errorSpy: jest.SpyInstance;
  const originalUrl = process.env.ALERTMANAGER_URL;
  const originalService = process.env.SERVICE_NAME;

  const payloadOf = (call = 0) => JSON.parse(fetchSpy.mock.calls[call][1].body);

  beforeEach(() => {
    service = new AlertService();
    fetchSpy = jest
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue({ ok: true, status: 200 } as unknown as Response);
    warnSpy = jest.spyOn(Logger.prototype, 'warn').mockImplementation();
    logSpy = jest.spyOn(Logger.prototype, 'log').mockImplementation();
    errorSpy = jest.spyOn(Logger.prototype, 'error').mockImplementation();
    process.env.ALERTMANAGER_URL = 'http://alertmanager:9093';
    delete process.env.SERVICE_NAME;
  });

  afterEach(() => {
    jest.restoreAllMocks();
    if (originalUrl === undefined) {
      delete process.env.ALERTMANAGER_URL;
    } else {
      process.env.ALERTMANAGER_URL = originalUrl;
    }
    if (originalService === undefined) {
      delete process.env.SERVICE_NAME;
    } else {
      process.env.SERVICE_NAME = originalService;
    }
  });

  it('returns false and warns when no Alertmanager url is configured', async () => {
    delete process.env.ALERTMANAGER_URL;

    const result = await service.triggerAlert('Disk full', '95% used');

    expect(result).toBe(false);
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalledWith(
      'Alert triggered but no ALERTMANAGER_URL is configured: [WARNING] Disk full - 95% used',
    );
    expect(logSpy).not.toHaveBeenCalled();
    expect(errorSpy).not.toHaveBeenCalled();
  });

  it('posts an Alertmanager alert to /api/v2/alerts on success', async () => {
    const result = await service.triggerAlert(
      'Order spike',
      '1000 orders/min',
      'WARNING',
      'delivery-service',
    );

    expect(result).toBe(true);
    expect(fetchSpy).toHaveBeenCalledTimes(1);

    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe('http://alertmanager:9093/api/v2/alerts');
    expect(init.method).toBe('POST');
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' });
    expect(init.signal).toBeDefined();

    const [alert] = payloadOf();
    expect(alert.labels).toEqual({
      alertname: 'ApplicationAlert',
      severity: 'warning',
      source: 'application',
      service: 'delivery-service',
      title: 'Order spike',
    });
    expect(alert.annotations.summary).toBe('Order spike');
    expect(alert.annotations.description).toBe('1000 orders/min');
    expect(alert.startsAt).toMatch(/^\d{4}-\d{2}-\d{2}T/);

    expect(logSpy).toHaveBeenCalledWith(
      'Alert dispatched to Alertmanager: Order spike',
    );
    expect(errorSpy).not.toHaveBeenCalled();
  });

  it('maps the severity to a lowercase Alertmanager label', async () => {
    await service.triggerAlert('Db down', 'connection refused', 'CRITICAL');

    expect(payloadOf()[0].labels.severity).toBe('critical');
  });

  it('falls back to SERVICE_NAME for the service label', async () => {
    process.env.SERVICE_NAME = 'notification-service';

    await service.triggerAlert('Outbox stuck', 'pending rows growing');

    expect(payloadOf()[0].labels.service).toBe('notification-service');
  });

  it('strips a trailing slash from the Alertmanager url', async () => {
    process.env.ALERTMANAGER_URL = 'http://alertmanager:9093/';

    await service.triggerAlert('t', 'm');

    expect(fetchSpy.mock.calls[0][0]).toBe(
      'http://alertmanager:9093/api/v2/alerts',
    );
  });

  it('treats a non ok response as a failure', async () => {
    fetchSpy.mockResolvedValue({ ok: false, status: 500 } as unknown as Response);

    const result = await service.triggerAlert('Order spike', 'boom');

    expect(result).toBe(false);
    expect(errorSpy).toHaveBeenCalledWith(
      'Failed to dispatch automated alert to Alertmanager',
      expect.stringContaining('HTTP error! status: 500'),
    );
    expect(logSpy).not.toHaveBeenCalled();
  });

  it('returns false when the request rejects', async () => {
    fetchSpy.mockRejectedValue(new Error('ECONNREFUSED'));

    const result = await service.triggerAlert('Network blip', 'no route');

    expect(result).toBe(false);
    expect(errorSpy).toHaveBeenCalledWith(
      'Failed to dispatch automated alert to Alertmanager',
      expect.stringContaining('ECONNREFUSED'),
    );
    expect(logSpy).not.toHaveBeenCalled();
  });

  it('never throws out of triggerAlert', async () => {
    fetchSpy.mockImplementation(() => {
      throw new Error('sync failure');
    });

    await expect(
      service.triggerAlert('Sync fail', 'immediate throw'),
    ).resolves.toBe(false);
  });
});
