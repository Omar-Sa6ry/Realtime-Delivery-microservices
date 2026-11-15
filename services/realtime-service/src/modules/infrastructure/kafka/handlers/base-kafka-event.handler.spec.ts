import { BaseKafkaEventHandler } from './base-kafka-event.handler';
import { EventDeduplicator } from '../../../features/events/event-deduplicator';
import { EventMapper } from '../../../features/events/event.mapper';
import { NatsPublisher } from '../../nats/nats.publisher';
import { RealtimeMetricsService } from '../../../../common/metrics/realtime-metrics.service';
import { RealtimeNatsSubjects } from '@delivery-micro/shard';

class TestHandler extends BaseKafkaEventHandler<{ v: number }> {
  readonly eventType = 'test.event';
  readonly natsSubject = RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED;
}

describe('BaseKafkaEventHandler', () => {
  let handler: TestHandler;
  let deduplicator: { isDuplicate: jest.Mock };
  let mapper: { toClientEvent: jest.Mock };
  let natsPublisher: { publish: jest.Mock };
  let metrics: { kafkaProcessed: { inc: jest.Mock } };

  const envelope = {
    eventId: 'evt-1',
    eventType: 'test.event',
    timestamp: Date.now(),
    payload: { v: 1 },
  };

  beforeEach(() => {
    deduplicator = { isDuplicate: jest.fn().mockResolvedValue(false) };
    mapper = {
      toClientEvent: jest.fn().mockReturnValue({ type: 'X', data: {} }),
    };
    natsPublisher = { publish: jest.fn().mockResolvedValue(true) };
    metrics = { kafkaProcessed: { inc: jest.fn() } };

    handler = new TestHandler(
      deduplicator as unknown as EventDeduplicator,
      mapper as unknown as EventMapper,
      natsPublisher as unknown as NatsPublisher,
      metrics as unknown as RealtimeMetricsService,
    );
    jest.spyOn((handler as any).logger, 'debug').mockImplementation();
    jest.spyOn((handler as any).logger, 'warn').mockImplementation();
  });

  it('maps and publishes unique events to the NATS subject', async () => {
    await handler.handle(envelope as any);

    expect(deduplicator.isDuplicate).toHaveBeenCalledWith('evt-1');
    expect(mapper.toClientEvent).toHaveBeenCalledWith(envelope);
    expect(natsPublisher.publish).toHaveBeenCalledWith(
      RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED,
      { type: 'X', data: {} },
    );
    expect(metrics.kafkaProcessed.inc).toHaveBeenCalledWith({
      eventType: 'test.event',
    });
  });

  it('skips duplicate events without publishing', async () => {
    deduplicator.isDuplicate.mockResolvedValue(true);

    await handler.handle(envelope as any);

    expect(mapper.toClientEvent).not.toHaveBeenCalled();
    expect(natsPublisher.publish).not.toHaveBeenCalled();
    expect(metrics.kafkaProcessed.inc).not.toHaveBeenCalled();
  });

  it('still counts processed events when the publish fails', async () => {
    const warn = jest.spyOn((handler as any).logger, 'warn');
    natsPublisher.publish.mockResolvedValue(false);

    await handler.handle(envelope as any);

    expect(warn).toHaveBeenCalled();
    expect(metrics.kafkaProcessed.inc).toHaveBeenCalled();
  });

  it('exposes the validate hook for subclasses', () => {
    expect((handler as any).validate({ a: 1 })).toEqual({ a: 1 });
  });
});
