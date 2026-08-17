import { Injectable, Logger } from '@nestjs/common';
import { RealtimeNatsSubjects, RealtimeMessage } from '@delivery/common';
import { RealtimeNatsService } from './nats.service';
import { RealtimeMetricsService } from '../../common/metrics/realtime-metrics.service';

@Injectable()
export class NatsPublisher {
  private readonly logger = new Logger(NatsPublisher.name);

  constructor(
    private readonly nats: RealtimeNatsService,
    private readonly metrics: RealtimeMetricsService,
  ) {}

  /** Publishes an event envelope to a NATS subject. Returns false when NATS is down. */
  async publish(
    subject: RealtimeNatsSubjects | string,
    data: unknown,
  ): Promise<boolean> {
    if (!this.nats.isConnected()) {
      this.logger.warn(`NATS not connected; dropping publish to ${subject}`);
      return false;
    }
    try {
      this.nats.getClient()!.publish(subject, this.nats.getCodec().encode(data));
      this.metrics.natsPublished.inc({ subject });
      return true;
    } catch (err) {
      this.logger.error(`NATS publish failed [${subject}]: ${err.message}`);
      return false;
    }
  }
}