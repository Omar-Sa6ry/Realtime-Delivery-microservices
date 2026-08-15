import { Injectable, Logger, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { NotificationOutbox, OutboxStatus } from '../../common/database/entities/notification-outbox.entity';
import { ClientProxy, ClientProxyFactory, Transport } from '@nestjs/microservices';
import { ConfigService } from '@nestjs/config';
import { NotificationNatsSubjects } from '@delivery/common';

@Injectable()
export class OutboxWorkerService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(OutboxWorkerService.name);
  private timer: NodeJS.Timeout;
  private natsClient: ClientProxy;

  constructor(
    @InjectRepository(NotificationOutbox)
    private outboxRepository: Repository<NotificationOutbox>,
    private configService: ConfigService,
  ) {
    this.natsClient = ClientProxyFactory.create({
      transport: Transport.NATS,
      options: {
        servers: [this.configService.get<string>('NATS_URL', 'nats://nats-srv:4222')],
        queue: 'notification-service',
      },
    });
  }

  async onModuleInit() {
    await this.natsClient.connect();
    this.timer = setInterval(() => this.processOutbox(), 5000);
  }

  async onModuleDestroy() {
    if (this.timer) clearInterval(this.timer);
    await this.natsClient.close();
  }

  private async processOutbox() {
    try {
      // Find up to 50 pending outbox records
      const records = await this.outboxRepository.find({
        where: { status: OutboxStatus.PENDING },
        take: 50,
        order: { createdAt: 'ASC' },
      });

      if (records.length === 0) return;

      for (const record of records) {
        try {
          const payload = record.payload;
          const subject = `${NotificationNatsSubjects.NOTIFICATION_USER}.${payload.userId}`;
          
          this.natsClient.emit(subject, payload);

          record.status = OutboxStatus.PUBLISHED;
          record.publishedAt = new Date();
          await this.outboxRepository.save(record);
        } catch (error) {
          this.logger.error(`Failed to publish outbox record ${record.id}: ${error.message}`);
          record.attemptCount += 1;
          if (record.attemptCount > 5) {
            record.status = OutboxStatus.FAILED;
          }
          await this.outboxRepository.save(record);
        }
      }
    } catch (error) {
      this.logger.error(`Error processing outbox: ${error.message}`, error.stack);
    }
  }
}
