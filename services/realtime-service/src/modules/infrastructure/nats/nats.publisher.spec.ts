import { NatsPublisher } from './nats.publisher';
import { RealtimeNatsService } from './nats.service';
import { RealtimeMetricsService } from '../../../common/metrics/realtime-metrics.service';
import { RealtimeNatsSubjects } from '@delivery-micro/shard';

describe('NatsPublisher', () => {
  let publisher: NatsPublisher;
  let nats: {
    isConnected: jest.Mock;
    getClient: jest.Mock;
    getCodec: jest.Mock;
  };
  let metrics: { natsPublished: { inc: jest.Mock } };
  let client: { publish: jest.Mock };
  let codec: { encode: jest.Mock };

  beforeEach(() => {
    client = { publish: jest.fn() };
    codec = { encode: jest.fn((d) => Buffer.from(JSON.stringify(d))) };
    nats = {
      isConnected: jest.fn().mockReturnValue(true),
      getClient: jest.fn().mockReturnValue(client),
      getCodec: jest.fn().mockReturnValue(codec),
    };
    metrics = { natsPublished: { inc: jest.fn() } };

    publisher = new NatsPublisher(
      nats as unknown as RealtimeNatsService,
      metrics as unknown as RealtimeMetricsService,
    );
  });

  it('publishes encoded payloads and records the metric', async () => {
    await expect(
      publisher.publish(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED, {
        a: 1,
      }),
    ).resolves.toBe(true);

    expect(codec.encode).toHaveBeenCalledWith({ a: 1 });
    expect(client.publish).toHaveBeenCalledWith(
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
      expect.anything(),
    );
    expect(metrics.natsPublished.inc).toHaveBeenCalledWith({
      subject: RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
    });
  });

  it('returns false and drops the message when NATS is down', async () => {
    nats.isConnected.mockReturnValue(false);

    await expect(publisher.publish('subject', {})).resolves.toBe(false);
    expect(client.publish).not.toHaveBeenCalled();
  });

  it('returns false when publishing throws', async () => {
    const errorLog = jest.spyOn((publisher as any).logger, 'error');
    client.publish.mockImplementation(() => {
      throw new Error('nats exploded');
    });

    await expect(publisher.publish('subject', {})).resolves.toBe(false);
    expect(errorLog).toHaveBeenCalled();
  });
});
