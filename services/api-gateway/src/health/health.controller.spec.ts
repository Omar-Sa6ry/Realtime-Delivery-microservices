import { HealthController } from './health.controller';
import { HealthService } from '@delivery-micro/shard';

describe('HealthController', () => {
  let controller: HealthController;
  let healthService: { getSystemStats: jest.Mock };

  beforeEach(() => {
    healthService = {
      getSystemStats: jest.fn().mockReturnValue({
        uptime: 12.34,
        memory: {
          heapTotalMemoryMB: 100,
          heapUsedMemoryMB: 50,
          rssMB: 200,
        },
        cpuUsage: { user: 1, system: 2 },
      }),
    };
    controller = new HealthController(
      healthService as unknown as HealthService,
    );
  });

  it('returns status ok and the api-gateway service name', () => {
    const res = controller.checkHealth();

    expect(res.status).toBe('ok');
    expect(res.service).toBe('api-gateway');
  });

  it('includes an ISO timestamp', () => {
    const res = controller.checkHealth();

    expect(new Date(res.timestamp).toISOString()).toBe(res.timestamp);
  });

  it('delegates system stats to HealthService', () => {
    const res = controller.checkHealth();

    expect(healthService.getSystemStats).toHaveBeenCalledTimes(1);
    expect(res.system).toEqual({
      uptime: 12.34,
      memory: { heapTotalMemoryMB: 100, heapUsedMemoryMB: 50, rssMB: 200 },
      cpuUsage: { user: 1, system: 2 },
    });
  });
});
