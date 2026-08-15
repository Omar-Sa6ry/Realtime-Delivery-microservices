import { Injectable, OnModuleInit, OnModuleDestroy, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Kafka, Consumer } from 'kafkajs';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { NotificationInbox } from '../../common/database/entities/notification-inbox.entity';
import { EventHandlerFactory } from './event-handlers/event-handler.factory';
import { KafkaEventPayload } from './event-handlers/event-handler.interface';
import { DeliveryKafkaTopics, PaymentKafkaTopics } from '@delivery/common';

@Injectable()
export class KafkaConsumer implements OnModuleInit, OnModuleDestroy {
  private consumer: Consumer;
  private readonly logger = new Logger(KafkaConsumer.name);

  constructor(
    private configService: ConfigService,
    @InjectRepository(NotificationInbox)
    private inboxRepository: Repository<NotificationInbox>,
    private eventHandlerFactory: EventHandlerFactory,
  ) {
    const kafka = new Kafka({
      clientId: 'notification-service',
      brokers: [this.configService.get<string>('KAFKA_BROKERS', 'kafka-srv:9092')],
    });

    this.consumer = kafka.consumer({ 
      groupId: this.configService.get<string>('KAFKA_GROUP_ID', 'notification-service'),
    });
  }

  async onModuleInit() {
    await this.consumer.connect();
    this.logger.log('Kafka Consumer connected');

    // Subscribe to topics
    const topics = [
      ...Object.values(DeliveryKafkaTopics),
      ...Object.values(PaymentKafkaTopics),
    ];

    for (const topic of topics) {
      await this.consumer.subscribe({ topic, fromBeginning: false });
    }

    await this.consumer.run({
      eachMessage: async ({ topic, partition, message }) => {
        try {
          if (!message.value) return;

          const payload: KafkaEventPayload = JSON.parse(message.value.toString());
          const eventId = payload.eventId || payload.id; // Ensure eventId exists
          const eventType = payload.eventType || topic;

          if (!eventId) {
            this.logger.warn(`Received message without eventId on topic ${topic}`);
            return;
          }

          // Idempotency Check (Inbox Pattern)
          const existing = await this.inboxRepository.findOne({
            where: { eventId, consumer: 'notification-service' },
          });

          if (existing) {
            this.logger.log(`Event ${eventId} already processed, skipping`);
            return;
          }

          const handler = this.eventHandlerFactory.getHandler(eventType);
          if (handler) {
            await handler.handle(payload);
          } else {
            this.logger.debug(`No handler for event type: ${eventType}`);
          }

          // Save to Inbox
          await this.inboxRepository.save({
            eventId,
            eventType,
            consumer: 'notification-service',
            processedAt: new Date(),
          });
          
        } catch (error) {
          this.logger.error(`Error processing message from topic ${topic}: ${error.message}`, error.stack);
        }
      },
    });
  }

  async onModuleDestroy() {
    await this.consumer.disconnect();
  }
}
