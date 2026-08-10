import { Processor, WorkerHost } from '@nestjs/bullmq';
import { Job } from 'bullmq';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { OutboxOrmEntity } from '../database/entities/outbox.orm-entity';
import { ClientProxy, ClientProxyFactory, Transport } from '@nestjs/microservices';
import { ConfigService } from '@nestjs/config';
import { Logger } from '@nestjs/common';

@Processor('outbox-queue')
export class OutboxProcessor extends WorkerHost {
  private readonly logger = new Logger(OutboxProcessor.name);
  private client: ClientProxy;

  constructor(
    @InjectRepository(OutboxOrmEntity)
    private readonly outboxRepo: Repository<OutboxOrmEntity>,
    private readonly configService: ConfigService,
  ) {
    super();
    const natsUrl = this.configService.get<string>('NATS_URL', 'nats://localhost:4222');
    this.client = ClientProxyFactory.create({
      transport: Transport.NATS,
      options: {
        servers: [natsUrl],
        queue: 'user-service',
      },
    });
  }

  async process(job: Job<any, any, string>): Promise<any> {
    const { outboxId } = job.data;
    const event = await this.outboxRepo.findOne({ where: { id: outboxId } });
    
    if (!event || event.processed) {
      return;
    }

    try {
      await this.client.connect();
      
      // Publish event to NATS
      this.client.emit(event.eventType, event.payload);
      
      // Mark as processed
      event.processed = true;
      await this.outboxRepo.save(event);
      
      this.logger.log(`Outbox event [${event.eventType}] processed via BullMQ for aggregate ${event.aggregateId}`);
    } catch (err) {
      this.logger.error(`Failed to process outbox event ${outboxId} via BullMQ`, err);
      throw err;
    }
  }
}
