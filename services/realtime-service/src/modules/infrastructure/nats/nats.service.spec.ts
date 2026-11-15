import { RealtimeNatsService } from './nats.service';
import { ConfigService } from '@nestjs/config';

const mockConnect = jest.fn();
const mockJSONCodec = jest.fn(() => ({
  encode: jest.fn(),
  decode: jest.fn(),
}));

jest.mock('nats', () => ({
  connect: (...args: any[]) => mockConnect(...args),
  JSONCodec: (...args: any[]) => mockJSONCodec(...args),
}));

describe('RealtimeNatsService', () => {
  let service: RealtimeNatsService;
  let config: { get: jest.Mock };
  let fakeClient: {
    isClosed: jest.Mock;
    status: jest.Mock;
    drain: jest.fn;
    close: jest.fn;
    publish: jest.Mock;
    subscribe: jest.Mock;
  };

  const env = { natsUrl: 'nats://localhost:4222', instanceId: 'rt-1' };

  beforeEach(() => {
    jest.clearAllMocks();
    jest.useFakeTimers();
    fakeClient = {
      isClosed: jest.fn().mockReturnValue(false),
      status: jest.fn().mockReturnValue(
        (async function* () {
          /* never yields */
        })(),
      ),
      drain: jest.fn().mockResolvedValue(undefined),
      close: jest.fn().mockResolvedValue(undefined),
      publish: jest.fn(),
      subscribe: jest.fn(),
    };
    mockConnect.mockResolvedValue(fakeClient);
    config = {
      get: jest.fn((key: string) => (key === 'realtime' ? env : undefined)),
    };
    service = new RealtimeNatsService(config as unknown as ConfigService);
  });

  afterEach(async () => {
    await service.onModuleDestroy();
    jest.useRealTimers();
  });

  it('connects on module init and reports connectivity', async () => {
    await service.onModuleInit();

    expect(mockConnect).toHaveBeenCalledWith(
      expect.objectContaining({
        servers: 'nats://localhost:4222',
        name: 'rt-1',
        maxReconnectAttempts: -1,
      }),
    );
    expect(service.isConnected()).toBe(true);
    expect(service.getClient()).toBe(fakeClient);
    expect(service.getLocalConnectionCount()).toBe(1);
  });

  it('exposes the JSON codec', () => {
    const codec = service.getCodec();
    expect(codec).toBeDefined();
    expect(mockJSONCodec).toHaveBeenCalled();
  });

  it('schedules a background reconnect when the initial connect fails', async () => {
    mockConnect
      .mockRejectedValueOnce(new Error('ECONNREFUSED'))
      .mockResolvedValue(fakeClient);
    const warn = jest.spyOn((service as any).logger, 'warn');

    await service.onModuleInit();

    expect(warn).toHaveBeenCalledWith(
      expect.stringContaining('Initial NATS connection failed'),
    );
    expect(service.isConnected()).toBe(false);

    await jest.advanceTimersByTimeAsync(3000);

    expect(mockConnect).toHaveBeenCalledTimes(2);
    expect(service.isConnected()).toBe(true);
  });

  it('keeps retrying while connect keeps failing', async () => {
    mockConnect
      .mockRejectedValueOnce(new Error('down'))
      .mockRejectedValueOnce(new Error('still down'))
      .mockResolvedValue(fakeClient);

    await service.onModuleInit();
    await jest.advanceTimersByTimeAsync(3000);
    expect(mockConnect).toHaveBeenCalledTimes(2);

    await jest.advanceTimersByTimeAsync(3000);
    expect(mockConnect).toHaveBeenCalledTimes(3);
    expect(service.isConnected()).toBe(true);
  });

  it('reports disconnected when the client is closed', async () => {
    await service.onModuleInit();
    fakeClient.isClosed.mockReturnValue(true);
    expect(service.isConnected()).toBe(false);
    expect(service.getLocalConnectionCount()).toBe(0);
  });

  it('tracks connectivity from NATS status updates', async () => {
    const statuses: Array<{ type: string } | Error> = [
      { type: 'update' },
      { type: 'disconnect' },
      { type: 'update' },
      new Error('status stream broke'),
    ];
    fakeClient.status.mockReturnValue(
      (async function* () {
        for (const s of statuses) {
          if (s instanceof Error) {
            throw s;
          }
          yield s;
        }
        await new Promise(() => {
          /* keep the stream open */
        });
      })(),
    );
    const warn = jest.spyOn((service as any).logger, 'warn');
    const log = jest.spyOn((service as any).logger, 'log');
    const error = jest.spyOn((service as any).logger, 'error');

    await service.onModuleInit();
    await jest.advanceTimersByTimeAsync(1);

    expect(log).toHaveBeenCalledWith('NATS reconnected');
    expect(warn).toHaveBeenCalledWith('NATS status changed: disconnect');
    expect(error).toHaveBeenCalledWith(
      expect.stringContaining('NATS status loop error'),
    );
  });

  it('drains and closes the connection on shutdown', async () => {
    await service.onModuleInit();
    await service.onModuleDestroy();

    expect(fakeClient.drain).toHaveBeenCalled();
    expect(fakeClient.close).toHaveBeenCalled();
    expect(service.getClient()).toBeNull();
    expect(service.isConnected()).toBe(false);
  });

  it('is a no-op on destroy when never connected', async () => {
    await expect(service.onModuleDestroy()).resolves.toBeUndefined();
  });
});
