import { Injectable } from '@nestjs/common';
import * as client from 'prom-client';

@Injectable()
export class RealtimeMetricsService {
  public readonly connectionCounter: client.Counter<string>;
  public readonly messagesReceived: client.Counter<string>;
  public readonly messagesSent: client.Counter<string>;
  public readonly messagesDropped: client.Counter<string>;
  public readonly backpressureSaturated: client.Counter<string>;
  public readonly natsPublished: client.Counter<string>;
  public readonly kafkaProcessed: client.Counter<string>;
  public readonly kafkaDlq: client.Counter<string>;
  public readonly locationUpdates: client.Counter<string>;
  public readonly staleConnections: client.Counter<string>;
  public readonly activeConnections: client.Gauge<string>;

  constructor() {
    const registry = client.register;
    const opts = <T>(name: string, help: string, labelNames: string[], type: string) => {
      const existing = registry.getSingleMetric(name) as T | undefined;
      return existing || (createByType(type, name, help, labelNames) as T);
    };
    const createByType = (type: string, name: string, help: string, labelNames: string[]) => {
      switch (type) {
        case 'counter':
          return new client.Counter({ name, help, labelNames, registers: [registry] });
        case 'gauge':
          return new client.Gauge({ name, help, labelNames, registers: [registry] });
        default:
          return new client.Counter({ name, help, labelNames, registers: [registry] });
      }
    };

    this.connectionCounter = opts('realtime_connections_total', 'Total WebSocket connections accepted/rejected', ['result'], 'counter');
    this.messagesReceived = opts('realtime_messages_received_total', 'Inbound client messages by type', ['type'], 'counter');
    this.messagesSent = opts('realtime_messages_sent_total', 'Outbound server messages by type/priority', ['type', 'priority'], 'counter');
    this.messagesDropped = opts('realtime_messages_dropped_total', 'Dropped outbound messages by reason', ['type', 'reason'], 'counter');
    this.backpressureSaturated = opts('realtime_backpressure_saturated_total', 'Sockets terminated due to slow consumer', [], 'counter');
    this.natsPublished = opts('realtime_nats_published_total', 'Messages published to NATS by subject', ['subject'], 'counter');
    this.kafkaProcessed = opts('realtime_kafka_processed_total', 'Kafka events processed', ['eventType'], 'counter');
    this.kafkaDlq = opts('realtime_kafka_dlq_total', 'Messages routed to DLQ by topic', ['topic'], 'counter');
    this.locationUpdates = opts('realtime_location_updates_total', 'Driver location updates accepted/rejected', ['result'], 'counter');
    this.staleConnections = opts('realtime_stale_connections_closed_total', 'Stale connections closed by heartbeat sweep', [], 'counter');
    this.activeConnections = opts('realtime_active_connections', 'Currently connected sockets on this node', [], 'gauge');
  }
}