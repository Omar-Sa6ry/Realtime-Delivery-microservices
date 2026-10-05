import { Logger } from '@nestjs/common';
import { AlertService } from './alert.service';

describe('AlertService', () => {
  let service: AlertService;
  let fetchSpy: jest.SpyInstance;
  let warnSpy: jest.SpyInstance;
  let logSpy: jest.SpyInstance;
  let errorSpy: jest.SpyInstance;
  const originalWebhook = process.env.ALERT_WEBHOOK_URL;

  beforeEach(() => {
    service = new AlertService();
    fetchSpy = jest
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue({ ok: true, status: 200 } as unknown as Response);
    warnSpy = jest.spyOn(Logger.prototype, 'warn').mockImplementation();
    logSpy = jest.spyOn(Logger.prototype, 'log').mockImplementation();
    errorSpy = jest.spyOn(Logger.prototype, 'error').mockImplementation();
    process.env.ALERT_WEBHOOK_URL = 'https://hooks.example.test/alerts';
  });

  afterEach(() => {
    jest.restoreAllMocks();
    if (originalWebhook === undefined) {
      delete process.env.ALERT_WEBHOOK_URL;
    } else {
      process.env.ALERT_WEBHOOK_URL = originalWebhook;
    }
  });

  it('returns false and warns when no webhook url is configured', async () => {
    delete process.env.ALERT_WEBHOOK_URL;

    const result = await service.triggerAlert('Disk full', '95% used');

    expect(result).toBe(false);
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalledWith(
      'Alert triggered but no ALERT_WEBHOOK_URL is configured: [WARNING] Disk full - 95% used',
    );
    expect(logSpy).not.toHaveBeenCalled();
    expect(errorSpy).not.toHaveBeenCalled();
  });

  it('posts the payload to the webhook on success', async () => {
    const result = await service.triggerAlert('Order spike', '1000 orders/min');

    expect(result).toBe(true);
    expect(fetchSpy).toHaveBeenCalledTimes(1);

    const [url, init] = fetchSpy.mock.calls[0];
    expect(url).toBe('https://hooks.example.test/alerts');
    expect(init.method).toBe('POST');
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' });

    const payload = JSON.parse(init.body);
    expect(payload.username).toBe('System Alert Bot');
    expect(payload.content).toContain('[WARNING] Order spike');
    expect(payload.content).toContain('1000 orders/min');
    expect(payload.content).toMatch(/Timestamp: \d{4}-\d{2}-\d{2}T/);

    expect(logSpy).toHaveBeenCalledWith(
      'Alert alert dispatched successfully: Order spike',
    );
    expect(errorSpy).not.toHaveBeenCalled();
  });

  it('uses the provided severity in the payload', async () => {
    await service.triggerAlert('Db down', 'connection refused', 'CRITICAL');

    const payload = JSON.parse(fetchSpy.mock.calls[0][1].body);
    expect(payload.content).toContain('[CRITICAL] Db down');
  });

  it('treats a non ok response as a failure', async () => {
    fetchSpy.mockResolvedValue({ ok: false, status: 500 } as unknown as Response);

    const result = await service.triggerAlert('Order spike', 'boom');

    expect(result).toBe(false);
    expect(errorSpy).toHaveBeenCalledWith(
      'Failed to dispatch automated webhook alert',
      expect.stringContaining('HTTP error! status: 500'),
    );
    expect(logSpy).not.toHaveBeenCalled();
  });

  it('returns false when the webhook request rejects', async () => {
    fetchSpy.mockRejectedValue(new Error('ECONNREFUSED'));

    const result = await service.triggerAlert('Network blip', 'no route');

    expect(result).toBe(false);
    expect(errorSpy).toHaveBeenCalledWith(
      'Failed to dispatch automated webhook alert',
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
