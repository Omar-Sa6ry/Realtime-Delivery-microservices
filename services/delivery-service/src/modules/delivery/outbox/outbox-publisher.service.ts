import {
  Injectable,
  Logger,
  OnModuleDestroy,
  OnModuleInit,
} from '@nestjs/common';
import { OutboxRepository } from './outbox.repository';
import { KafkaProducer } from './kafka.producer';
import { Outbox } from '../entities/outbox.entity';

@Injectable()
export class OutboxPublisherService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(OutboxPublisherService.name);
  private timer?: NodeJS.Timeout;
  private running = false;
  constructor(
    private readonly outbox: OutboxRepository,
    private readonly producer: KafkaProducer,
  ) {}

  onModuleInit(): void {
    this.timer = setInterval(() => {
      void this.publishBatch();
    }, this.intervalMs);
    void this.publishBatch();
  }

  onModuleDestroy(): void {
    if (this.timer) clearInterval(this.timer);
  }

  private get intervalMs(): number {
    return Math.max(
      250,
      Number(process.env.OUTBOX_PUBLISH_INTERVAL_MS ?? 1000),
    );
  }

  private get batchSize(): number {
    return Math.min(
      100,
      Math.max(1, Number(process.env.OUTBOX_BATCH_SIZE ?? 25)),
    );
  }

  async publishBatch(): Promise<void> {
    if (this.running) return;
    this.running = true;
    try {
      for (const event of await this.outbox.claimPending(this.batchSize))
        await this.publish(event);
    } finally {
      this.running = false;
    }
  }
  
  private async publish(event: Outbox): Promise<void> {
    try {
      await this.producer.publish(event);
      await this.outbox.markPublished(event);
    } catch (error) {
      const failure = error instanceof Error ? error : new Error(String(error));
      const retryMs = Math.min(300000, 1000 * 2 ** Math.min(event.attempts, 8));
      await this.outbox.markFailed(event, failure, retryMs);
      this.logger.error(failure.message);
    }
  }
}
