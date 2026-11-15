import realtimeConfig from './realtime.config';

describe('realtime.config', () => {
  const originalEnv = process.env;

  afterEach(() => {
    process.env = originalEnv;
  });

  it('exposes the namespace realtime', () => {
    expect((realtimeConfig as any).KEY).toBe('CONFIGURATION(realtime)');
  });

  it('provides defaults when env vars are missing', () => {
    delete process.env.NATS_URL;
    delete process.env.INSTANCE_ID;
    delete process.env.DELIVERY_SERVICE_GRPC_URL;
    delete process.env.DRIVER_SERVICE_GRPC_URL;

    const cfg = (realtimeConfig as any)();
    expect(cfg.natsUrl).toBe('nats://localhost:4222');
    expect(cfg.instanceId).toBe(`realtime-${process.pid}`);
    expect(cfg.authzDeliveryServiceUrl).toBe('delivery-srv:50054');
    expect(cfg.authzDriverServiceUrl).toBe('driver-srv:50055');
  });

  it('reads values from environment variables', () => {
    process.env.NATS_URL = 'nats://nats:4222';
    process.env.INSTANCE_ID = 'realtime-1';
    process.env.DELIVERY_SERVICE_GRPC_URL = 'delivery:50054';
    process.env.DRIVER_SERVICE_GRPC_URL = 'driver:50055';

    const cfg = (realtimeConfig as any)();
    expect(cfg.natsUrl).toBe('nats://nats:4222');
    expect(cfg.instanceId).toBe('realtime-1');
    expect(cfg.authzDeliveryServiceUrl).toBe('delivery:50054');
    expect(cfg.authzDriverServiceUrl).toBe('driver:50055');
  });
});
