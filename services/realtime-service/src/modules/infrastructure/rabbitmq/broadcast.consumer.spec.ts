import { RealtimeBroadcastConsumer } from './broadcast.consumer';
import { RabbitMQService } from '@delivery-micro/shard';
import { EventMapper } from '../../features/events/event.mapper';
import { NatsPublisher } from '../nats/nats.publisher';

describe('RealtimeBroadcastConsumer', () => {
  let consumer: RealtimeBroadcastConsumer;
  let rabbitmq: Record<string, never>;
  let mapper: { toClientEvent: jest.Mock };
  let natsPublisher: { publish: jest.Mock };

  const envelope = {
    eventId: 'evt-1',
    eventType: 'delivery.created',
    timestamp: 123,
    payload: { deliveryId: 'd1' },
  };
  const ctx = { msg: {}, attempt: 1, traceId: 'trace-1' } as any;

  beforeEach(() => {
    mapper = {
      toClientEvent: jest.fn().mockReturnValue({ type: 'X', data: {} }),
    };
    natsPublisher = { publish: jest.fn().mockResolvedValue(true) };
    rabbitmq = {};
  });

  const build = (withDeps = true) =>
    new RealtimeBroadcastConsumer(
      rabbitmq as RabbitMQService,
      withDeps ? (mapper as unknown as EventMapper) : undefined,
      withDeps ? (natsPublisher as unknown as NatsPublisher) : undefined,
    );

  beforeEach(() => {
    consumer = build();
    jest.spyOn((consumer as any).logger, 'warn').mockImplementation();
    jest.spyOn((consumer as any).logger, 'debug').mockImplementation();
  });

  it('maps the envelope and publishes to the delivery status subject', async () => {
    await (consumer as any).handleEvent(envelope, ctx);

    expect(mapper.toClientEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        eventId: 'evt-1',
        eventType: 'delivery.created',
        traceId: 'trace-1',
        payload: { deliveryId: 'd1' },
      }),
    );
    expect(natsPublisher.publish).toHaveBeenCalledWith(
      'realtime.delivery.status.updated',
      { type: 'X', data: {} },
    );
  });

  it('falls back to the envelope traceId when the context has none', async () => {
    await (consumer as any).handleEvent({ ...envelope, traceId: 'env-trace' }, {
      msg: {},
      attempt: 1,
    } as any);

    expect(mapper.toClientEvent).toHaveBeenCalledWith(
      expect.objectContaining({ traceId: 'env-trace' }),
    );
  });

  it('defaults the timestamp when missing', async () => {
    await (consumer as any).handleEvent(
      { ...envelope, timestamp: undefined as any },
      ctx,
    );

    expect(mapper.toClientEvent).toHaveBeenCalledWith(
      expect.objectContaining({ timestamp: expect.any(Number) }),
    );
  });

  it('warns and swallows mapping/publish failures (acks to avoid DLQ storm)', async () => {
    const warn = jest.spyOn((consumer as any).logger, 'warn');
    mapper.toClientEvent.mockImplementation(() => {
      throw new Error('unsupported');
    });

    await expect(
      (consumer as any).handleEvent(envelope, ctx),
    ).resolves.toBeUndefined();
    expect(warn).toHaveBeenCalled();

    mapper.toClientEvent.mockReturnValue({ type: 'X', data: {} });
    natsPublisher.publish.mockRejectedValue(new Error('nats down'));
    await expect(
      (consumer as any).handleEvent(envelope, ctx),
    ).resolves.toBeUndefined();
    expect(warn).toHaveBeenCalledTimes(2);
  });

  it('warns when the optional dependencies are missing', async () => {
    const bare = build(false);
    const warn = jest.spyOn((bare as any).logger, 'warn');

    await expect(
      (bare as any).handleEvent(envelope, ctx),
    ).resolves.toBeUndefined();
    expect(warn).toHaveBeenCalled();
  });

  it('reports n/a in the missing-deps warning when the context has no traceId', async () => {
    const bare = build(false);
    const warn = jest.spyOn((bare as any).logger, 'warn');

    await expect(
      (bare as any).handleEvent(envelope, { msg: {}, attempt: 1 } as any),
    ).resolves.toBeUndefined();

    expect(warn).toHaveBeenCalledWith(expect.stringContaining('trace=n/a'));
  });

  it('defaults an undefined envelope payload to an empty object', async () => {
    await (consumer as any).handleEvent(
      { ...envelope, payload: undefined as any },
      ctx,
    );

    expect(mapper.toClientEvent).toHaveBeenCalledWith(
      expect.objectContaining({ payload: {} }),
    );
  });

  it('is intentionally not idempotent (at-most-once per session)', async () => {
    await expect((consumer as any).isDuplicate('evt-1')).resolves.toBe(false);
  });
});
