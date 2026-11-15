import { DeliveryPolicy } from './delivery.policy';
import { GrpcClient } from '../../../infrastructure/grpc/grpc.client';
import { RedisService } from '@bts-soft/cache';

describe('DeliveryPolicy', () => {
  let policy: DeliveryPolicy;
  let grpc: {
    isDeliveryClientConfigured: jest.Mock;
    callDelivery: jest.Mock;
  };
  let redis: { get: jest.Mock; set: jest.Mock };

  beforeEach(() => {
    grpc = {
      isDeliveryClientConfigured: jest.fn().mockReturnValue(true),
      callDelivery: jest.fn(),
    };
    redis = {
      get: jest.fn().mockResolvedValue(null),
      set: jest.fn().mockResolvedValue(undefined),
    };
    policy = new DeliveryPolicy(
      grpc as unknown as GrpcClient,
      redis as unknown as RedisService,
    );
    jest.restoreAllMocks();
  });

  it('returns the cached result when present', async () => {
    redis.get.mockResolvedValue({ isParticipant: true });
    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(true);
    expect(grpc.callDelivery).not.toHaveBeenCalled();
  });

  it('returns cached false results too', async () => {
    redis.get.mockResolvedValue({ isParticipant: false });
    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(false);
  });

  it('denies when the delivery gRPC client is not configured', async () => {
    grpc.isDeliveryClientConfigured.mockReturnValue(false);
    const warn = jest.spyOn((policy as any).logger, 'warn');

    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(false);
    expect(warn).toHaveBeenCalled();
    expect(grpc.callDelivery).not.toHaveBeenCalled();
  });

  it('queries gRPC and caches positive results', async () => {
    grpc.callDelivery.mockResolvedValue({ isParticipant: true });

    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(true);
    expect(grpc.callDelivery).toHaveBeenCalledWith('IsParticipant', {
      userId: 'u1',
      deliveryId: 'd1',
    });
    expect(redis.set).toHaveBeenCalledWith(
      'realtime:authz:u1:d1',
      { isParticipant: true },
      30,
    );
  });

  it('does not cache negative results', async () => {
    grpc.callDelivery.mockResolvedValue({ isParticipant: false });

    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(false);
    expect(redis.set).not.toHaveBeenCalled();
  });

  it('treats non-true gRPC responses as not a participant', async () => {
    grpc.callDelivery.mockResolvedValue({ isParticipant: 'yes' });
    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(false);

    grpc.callDelivery.mockResolvedValue(null);
    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(false);
  });

  it('denies when the gRPC call throws', async () => {
    jest.spyOn((policy as any).logger, 'error').mockImplementation();
    grpc.callDelivery.mockRejectedValue(new Error('grpc down'));

    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(false);
  });

  it('ignores redis cache failures and still queries gRPC', async () => {
    redis.get.mockRejectedValue(new Error('redis down'));
    grpc.callDelivery.mockResolvedValue({ isParticipant: true });

    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(true);
  });

  it('ignores cache write failures for positive results', async () => {
    redis.set.mockRejectedValue(new Error('redis down'));
    grpc.callDelivery.mockResolvedValue({ isParticipant: true });

    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(true);
  });

  it('ignores redis get rejections when reading cache', async () => {
    redis.get.mockRejectedValue(new Error('redis down'));
    grpc.callDelivery.mockResolvedValue({ isParticipant: false });
    await expect(policy.isParticipant('u1', 'd1')).resolves.toBe(false);
  });
});
