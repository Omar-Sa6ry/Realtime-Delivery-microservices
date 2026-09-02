import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Consumer, EachMessagePayload } from 'kafkajs';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { NotificationInbox } from '../../common/database/entities/notification-inbox.entity';
import { EventHandlerFactory } from './event-handlers/event-handler.factory';
import { KafkaEventPayload } from './event-handlers/event-handler.interface';
import {
  DeliveryKafkaTopics,
  PaymentKafkaTopics,
  MediaKafkaTopics,
  UserKafkaTopics,
  KafkaService,
  BaseKafkaConsumer,
} from '@delivery/common';

@Injectable()
export class KafkaConsumer extends BaseKafkaConsumer {
  protected consumer: Consumer;
  protected readonly logger: any = new Logger(KafkaConsumer.name);
  protected topics = [
    ...Object.values(DeliveryKafkaTopics),
    ...Object.values(PaymentKafkaTopics),
    ...Object.values(MediaKafkaTopics),
    ...Object.values(UserKafkaTopics),
  ];

  constructor(
    private configService: ConfigService,
    @InjectRepository(NotificationInbox)
    private inboxRepository: Repository<NotificationInbox>,
    private eventHandlerFactory: EventHandlerFactory,
    protected kafkaService: KafkaService,
  ) {
    super();
    this.consumer = this.kafkaService.consumer(
      this.configService.get<string>('KAFKA_GROUP_ID', 'notification-service'),
    );
  }

  protected async handleMessage({ topic, partition, message }: EachMessagePayload): Promise<void> {
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
      this.logger.error(`Error processing message from topic ${topic}: ${(error as Error).message}`, (error as Error).stack);
    }
  }
}
