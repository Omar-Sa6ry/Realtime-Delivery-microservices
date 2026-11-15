import { DriverPolicy } from './driver.policy';
import { GrpcClient } from '../../../infrastructure/grpc/grpc.client';
import { RedisService } from '@bts-soft/cache';

describe('DriverPolicy', () => {
  let policy: DriverPolicy;
  let grpc: {
    isDriverClientConfigured: jest.Mock;
    callDriver: jest.Mock;
  };
  let redis: { get: jest.Mock; set: jest.Mock };

  beforeEach(() => {
    grpc = {
      isDriverClientConfigured: jest.fn().mockReturnValue(true),
      callDriver: jest.fn(),
    };
    redis = {
      get: jest.fn().mockResolvedValue(null),
      set: jest.fn().mockResolvedValue(undefined),
    };
    policy = new DriverPolicy(
      grpc as unknown as GrpcClient,
      redis as unknown as RedisService,
    );
    jest.restoreAllMocks();
  });

  it('returns the cached result when present', async () => {
    redis.get.mockResolvedValue({ isAssigned: true });
    await expect(policy.isAssignedDriver('drv1', 'd1')).resolves.toBe(true);
    expect(redis.get).toHaveBeenCalledWith('realtime:authz:driver:drv1:d1');
    expect(grpc.callDriver).not.toHaveBeenCalled();
  });

  it('denies when the driver gRPC client is not configured', async () => {
    grpc.isDriverClientConfigured.mockReturnValue(false);
    const warn = jest.spyOn((policy as any).logger, 'warn');

    await expect(policy.isAssignedDriver('drv1', 'd1')).resolves.toBe(false);
    expect(warn).toHaveBeenCalled();
  });

  it('queries gRPC and caches positive results', async () => {
    grpc.callDriver.mockResolvedValue({ isAssigned: true });

    await expect(policy.isAssignedDriver('drv1', 'd1')).resolves.toBe(true);
    expect(grpc.callDriver).toHaveBeenCalledWith('IsAssignedDriver', {
      driverId: 'drv1',
      deliveryId: 'd1',
    });
    expect(redis.set).toHaveBeenCalledWith(
      'realtime:authz:driver:drv1:d1',
      { isAssigned: true },
      30,
    );
  });

  it('does not cache negative results', async () => {
    grpc.callDriver.mockResolvedValue({ isAssigned: false });
    await expect(policy.isAssignedDriver('drv1', 'd1')).resolves.toBe(false);
    expect(redis.set).not.toHaveBeenCalled();
  });

  it('denies when the gRPC call throws', async () => {
    jest.spyOn((policy as any).logger, 'error').mockImplementation();
    grpc.callDriver.mockRejectedValue(new Error('grpc down'));
    await expect(policy.isAssignedDriver('drv1', 'd1')).resolves.toBe(false);
  });

  it('denies on unexpected response shapes', async () => {
    grpc.callDriver.mockResolvedValue(undefined);
    await expect(policy.isAssignedDriver('drv1', 'd1')).resolves.toBe(false);
  });

  it('survives cache read/write failures', async () => {
    redis.get.mockRejectedValue(new Error('down'));
    redis.set.mockRejectedValue(new Error('down'));
    grpc.callDriver.mockResolvedValue({ isAssigned: true });

    await expect(policy.isAssignedDriver('drv1', 'd1')).resolves.toBe(true);
  });
});
