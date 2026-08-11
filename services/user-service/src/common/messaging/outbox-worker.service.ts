import { Injectable, Logger, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Outbox } from '../database/entities/outbox.entity';
import { ClientProxy, ClientProxyFactory, Transport } from '@nestjs/microservices';
import { ConfigService } from '@nestjs/config';
import { InjectQueue } from '@nestjs/bullmq';
import { Queue } from 'bullmq';

@Injectable()
export class OutboxWorkerService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(OutboxWorkerService.name);
  private client: ClientProxy;
  private timer: NodeJS.Timeout;

  constructor(
    @InjectRepository(Outbox)
    private readonly outboxRepo: Repository<Outbox>,
    private readonly configService: ConfigService,
    @InjectQueue('outbox-queue') private readonly outboxQueue: Queue,
  ) {
    const natsUrl = this.configService.get<string>('NATS_URL', 'nats://localhost:4222');
    this.client = ClientProxyFactory.create({
      transport: Transport.NATS,
      options: {
        servers: [natsUrl],
        queue: 'user-service',
      },
    });
  }

  async onModuleInit() {
    await this.client.connect();
    // Run reconciliation fallback check every 60 seconds (safe with SKIP LOCKED in multi-node clusters)
    this.timer = setInterval(() => this.reconcileOutbox(), 60000);
  }

  onModuleDestroy() {
    if (this.timer) clearInterval(this.timer);
    this.client.close();
  }

  async enqueueEvent(outboxId: string): Promise<void> {
    try {
      await this.outboxQueue.add('process-event', { outboxId }, { removeOnComplete: true });
      this.logger.debug(`Outbox event ${outboxId} enqueued to BullMQ`);
    } catch (err) {
      this.logger.error(`Failed to enqueue outbox event ${outboxId} to Redis`, err);
    }
  }

  public async reconcileOutbox(): Promise<void> {
    try {
      await this.outboxRepo.manager.transaction(async (transactionalEntityManager) => {
        const events = await transactionalEntityManager.find(Outbox, {
          where: { processed: false },
          order: { createdAt: 'ASC' },
          take: 20,
          lock: { mode: 'pessimistic_write', onLocked: 'skip_locked' },
        });

        if (events.length > 0) {
          this.logger.log(`Outbox reconciliation: found ${events.length} unprocessed events`);
        }

        for (const event of events) {
          try {
            await this.client.connect();
            this.client.emit(event.eventType, event.payload);
            
            event.processed = true;
            await transactionalEntityManager.save(event);
            
            this.logger.log(`Outbox event [${event.eventType}] reconciled via SKIP LOCKED for aggregate ${event.aggregateId}`);
          } catch (err) {
            this.logger.error(`Reconciliation failed for event ${event.id}`, err);
          }
        }
      });
    } catch (error) {
      this.logger.error('Error during outbox database reconciliation', error);
    }
  }
}
