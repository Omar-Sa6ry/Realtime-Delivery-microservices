import { RealtimeMetricsService } from './realtime-metrics.service';

describe('RealtimeMetricsService', () => {
  let service: RealtimeMetricsService;

  beforeEach(() => {
    service = new RealtimeMetricsService();
  });

  it('creates all prometheus metrics', () => {
    expect(service.connectionCounter).toBeDefined();
    expect(service.messagesReceived).toBeDefined();
    expect(service.messagesSent).toBeDefined();
    expect(service.messagesDropped).toBeDefined();
    expect(service.backpressureSaturated).toBeDefined();
    expect(service.natsPublished).toBeDefined();
    expect(service.kafkaProcessed).toBeDefined();
    expect(service.kafkaDlq).toBeDefined();
    expect(service.locationUpdates).toBeDefined();
    expect(service.staleConnections).toBeDefined();
    expect(service.activeConnections).toBeDefined();
  });

  it('is idempotent across instantiations (reuses registry metrics)', () => {
    const second = new RealtimeMetricsService();
    expect(second.connectionCounter).toBe(service.connectionCounter);
    expect(second.activeConnections).toBe(service.activeConnections);
  });

  it('increments counters with labels', () => {
    expect(() =>
      service.connectionCounter.inc({ result: 'accepted' }),
    ).not.toThrow();
    expect(() =>
      service.locationUpdates.inc({ result: 'accepted' }),
    ).not.toThrow();
    expect(() =>
      service.messagesSent.inc({ type: 'PONG', priority: 'NORMAL' }),
    ).not.toThrow();
  });

  it('sets gauge values', () => {
    expect(() => service.activeConnections.set(42)).not.toThrow();
  });
});
