import { GrpcClient } from './grpc.client';
import { ConfigService } from '@nestjs/config';

const mockLoadSync = jest.fn();
const mockLoadPackageDefinition = jest.fn();
const mockCreateInsecure = jest.fn(() => 'insecure-creds');

jest.mock('@grpc/proto-loader', () => ({
  loadSync: (...args: any[]) => mockLoadSync(...args),
}));

jest.mock('@grpc/grpc-js', () => ({
  loadPackageDefinition: (...args: any[]) => mockLoadPackageDefinition(...args),
  credentials: { createInsecure: () => mockCreateInsecure() },
}));

type RpcCallback = (err: any, resp: any) => void;

const instances: any[] = [];

function FakeGrpcClient(this: any, url: string, creds: any) {
  const instance = {
    url,
    creds,
    IsParticipant: jest.fn((_args: any, _opts: any, cb: RpcCallback) =>
      cb(null, { isParticipant: true }),
    ),
    IsAssignedDriver: jest.fn((_args: any, _opts: any, cb: RpcCallback) =>
      cb(null, { isAssigned: true }),
    ),
  };
  instances.push(instance);
  return instance;
}
(FakeGrpcClient as any).mock = { results: [] };

describe('GrpcClient', () => {
  let client: GrpcClient;

  const makeConfig = (delivery?: string, driver?: string) =>
    ({
      get: jest.fn((key: string) => {
        if (key === 'realtime') {
          return {
            natsUrl: 'nats://x',
            instanceId: 'i',
            authzDeliveryServiceUrl: delivery,
            authzDriverServiceUrl: driver,
          };
        }
        return undefined;
      }),
    }) as unknown as ConfigService;

  const lastInstance = () => instances[instances.length - 1];
  expect(typeof lastInstance).toBe('function');

  beforeEach(() => {
    jest.clearAllMocks();
    instances.length = 0;
    mockLoadSync.mockReturnValue({});
    mockLoadPackageDefinition.mockReturnValue({
      delivery: { DeliveryService: FakeGrpcClient },
      driver: { DriverService: FakeGrpcClient },
    });
    client = new GrpcClient(
      makeConfig('delivery-srv:50054', 'driver-srv:50055'),
    );
    jest.spyOn((client as any).logger, 'warn').mockImplementation();
    jest.spyOn((client as any).logger, 'error').mockImplementation();
    jest.spyOn((client as any).logger, 'log').mockImplementation();
  });

  describe('configuration probes', () => {
    it('reports configured endpoints', () => {
      expect(client.isDeliveryClientConfigured()).toBe(true);
      expect(client.isDriverClientConfigured()).toBe(true);
    });

    it('reports unconfigured endpoints', () => {
      const bare = new GrpcClient(makeConfig(undefined, undefined));
      expect(bare.isDeliveryClientConfigured()).toBe(false);
      expect(bare.isDriverClientConfigured()).toBe(false);
    });
  });

  describe('callDelivery', () => {
    it('returns null and warns when not configured', async () => {
      const bare = new GrpcClient(makeConfig(undefined, undefined));
      const warn = jest.spyOn((bare as any).logger, 'warn');

      await expect(bare.callDelivery('IsParticipant', {})).resolves.toBeNull();
      expect(warn).toHaveBeenCalled();
    });

    it('creates the client lazily and invokes the method', async () => {
      const result = await client.callDelivery('IsParticipant', {
        userId: 'u1',
        deliveryId: 'd1',
      });

      expect(mockLoadSync).toHaveBeenCalledWith(
        expect.stringContaining('delivery.proto'),
        expect.objectContaining({ keepCase: true }),
      );
      expect(result).toEqual({ isParticipant: true });
      expect(instances[0].url).toBe('delivery-srv:50054');
      expect(instances[0].creds).toBe('insecure-creds');
    });

    it('resolves null and records an error when the RPC fails', async () => {
      const instance = await warmUpDelivery(client);
      instance.IsParticipant.mockImplementation(
        (_a: any, _o: any, cb: RpcCallback) =>
          cb({ code: 14, details: 'unavailable' }, null),
      );

      await expect(
        client.callDelivery('IsParticipant', {}),
      ).resolves.toBeNull();
    });

    it('resolves null when the method does not exist on the client', async () => {
      await warmUpDelivery(client);

      await expect(client.callDelivery('Missing', {})).resolves.toBeNull();
    });

    it('reuses the cached client instance across calls', async () => {
      await client.callDelivery('IsParticipant', {});
      await client.callDelivery('IsParticipant', {});
      expect(instances).toHaveLength(1);
    });
  });

  describe('callDriver', () => {
    it('returns null and warns when not configured', async () => {
      const bare = new GrpcClient(makeConfig(undefined, undefined));
      await expect(bare.callDriver('IsAssignedDriver', {})).resolves.toBeNull();
    });

    it('invokes the driver service method', async () => {
      const result = await client.callDriver('IsAssignedDriver', {
        driverId: 'drv1',
        deliveryId: 'd1',
      });
      expect(result).toEqual({ isAssigned: true });
      expect(instances[0].url).toBe('driver-srv:50055');
    });

    it('resolves null on driver RPC errors', async () => {
      const instance = await warmUpDriver(client);
      instance.IsAssignedDriver.mockImplementation(
        (_a: any, _o: any, cb: RpcCallback) =>
          cb({ code: 4, details: 'not found' }, null),
      );
      await expect(
        client.callDriver('IsAssignedDriver', {}),
      ).resolves.toBeNull();
    });
  });

  describe('circuit breaker', () => {
    it('opens the delivery circuit after the failure threshold and recovers after cool-down', async () => {
      const instance = await warmUpDelivery(client);
      instance.IsParticipant.mockImplementation(
        (_a: any, _o: any, cb: RpcCallback) =>
          cb({ code: 14, details: 'down' }, null),
      );

      let now = 1_000_000;
      const nowSpy = jest.spyOn(Date, 'now').mockImplementation(() => now);

      for (let i = 0; i < 5; i++) {
        await client.callDelivery('IsParticipant', {});
      }

      // Circuit OPEN: the next call is short-circuited (no RPC invocation)
      const callsBefore = instance.IsParticipant.mock.calls.length;
      await expect(
        client.callDelivery('IsParticipant', {}),
      ).resolves.toBeNull();
      expect(instance.IsParticipant.mock.calls.length).toBe(callsBefore);

      // Cool-down elapses -> HALF_OPEN -> probe succeeds -> CLOSED
      now += 11_000;
      instance.IsParticipant.mockImplementation(
        (_a: any, _o: any, cb: RpcCallback) =>
          cb(null, { isParticipant: true }),
      );
      await expect(client.callDelivery('IsParticipant', {})).resolves.toEqual({
        isParticipant: true,
      });

      // Closed again: normal calls flow
      await client.callDelivery('IsParticipant', {});
      expect(instance.IsParticipant.mock.calls.length).toBeGreaterThan(
        callsBefore,
      );

      nowSpy.mockRestore();
    });

    it('opens the driver circuit, short-circuits while open, and recovers after cool-down', async () => {
      const instance = await warmUpDriver(client);
      instance.IsAssignedDriver.mockImplementation(
        (_a: any, _o: any, cb: RpcCallback) =>
          cb({ code: 4, details: 'down' }, null),
      );

      let now = 2_000_000;
      const nowSpy = jest.spyOn(Date, 'now').mockImplementation(() => now);

      for (let i = 0; i < 5; i++) {
        await expect(
          client.callDriver('IsAssignedDriver', {}),
        ).resolves.toBeNull();
      }

      // Driver circuit OPEN: short-circuited (no RPC invocation)
      const callsBefore = instance.IsAssignedDriver.mock.calls.length;
      await expect(
        client.callDriver('IsAssignedDriver', {}),
      ).resolves.toBeNull();
      expect(instance.IsAssignedDriver.mock.calls.length).toBe(callsBefore);

      // Cool-down elapses -> HALF_OPEN -> probe succeeds -> CLOSED
      now += 11_000;
      instance.IsAssignedDriver.mockImplementation(
        (_a: any, _o: any, cb: RpcCallback) => cb(null, { isAssigned: true }),
      );
      await expect(client.callDriver('IsAssignedDriver', {})).resolves.toEqual({
        isAssigned: true,
      });
      await expect(client.callDriver('IsAssignedDriver', {})).resolves.toEqual({
        isAssigned: true,
      });

      nowSpy.mockRestore();
    });

    it('keeps the delivery and driver circuits independent', async () => {
      const deliveryInstance = await warmUpDelivery(client);
      deliveryInstance.IsParticipant.mockImplementation(
        (_a: any, _o: any, cb: RpcCallback) =>
          cb({ code: 14, details: 'down' }, null),
      );

      for (let i = 0; i < 5; i++) {
        await client.callDelivery('IsParticipant', {});
      }

      // Delivery circuit open
      const before = deliveryInstance.IsParticipant.mock.calls.length;
      await client.callDelivery('IsParticipant', {});
      expect(deliveryInstance.IsParticipant.mock.calls.length).toBe(before);

      // Driver circuit unaffected
      await expect(client.callDriver('IsAssignedDriver', {})).resolves.toEqual({
        isAssigned: true,
      });
    });
  });
});

async function warmUpDelivery(client: GrpcClient): Promise<any> {
  await client.callDelivery('IsParticipant', {});
  return instances[instances.length - 1];
}

async function warmUpDriver(client: GrpcClient): Promise<any> {
  await client.callDriver('IsAssignedDriver', {});
  return instances[instances.length - 1];
}
