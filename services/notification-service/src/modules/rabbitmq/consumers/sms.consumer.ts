import { Injectable, Logger } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import {
  BaseRabbitMQConsumer,
  RabbitMQConsumeContext,
  RabbitMQEventEnvelope,
  RabbitMQExchanges,
  RabbitMQQueues,
  RabbitMQService,
} from '@delivery-micro/shard';
import { NotificationInbox } from '../../../common/database/entities/notification-inbox.entity';
import { EventHandlerFactory } from '../../kafka/event-handlers/event-handler.factory';

const CONSUMER_NAME = 'notification-service-rabbitmq';

@Injectable()
export class SmsRabbitMQConsumer extends BaseRabbitMQConsumer {
  protected readonly queue = RabbitMQQueues.NOTIFICATIONS_SMS;
  protected readonly exchange = RabbitMQExchanges.NOTIFICATIONS;
  protected readonly routingKeys = [''];
  protected readonly logger = new Logger(SmsRabbitMQConsumer.name);
  protected readonly prefetch = 20;

  constructor(
    rabbitmq: RabbitMQService,
    private readonly eventHandlerFactory: EventHandlerFactory,
    @InjectRepository(NotificationInbox)
    private readonly inboxRepository: Repository<NotificationInbox>,
  ) {
    super(rabbitmq);
  }

  protected async handleEvent(
    envelope: RabbitMQEventEnvelope,
    _ctx: RabbitMQConsumeContext,
  ): Promise<void> {
    const payload = this.toHandlerPayload(envelope);
    const handler = this.eventHandlerFactory.getHandler(envelope.eventType);
    if (handler) {
      await handler.handle(payload);
    } else {
      this.logger.debug(
        `No handler for RabbitMQ event type: ${envelope.eventType}`,
      );
    }

    await this.inboxRepository.save(
      this.inboxRepository.create({
        eventId: envelope.eventId,
        eventType: envelope.eventType,
        consumer: CONSUMER_NAME,
        processedAt: new Date(),
      }),
    );
  }

  protected override async isDuplicate(eventId: string): Promise<boolean> {
    const existing = await this.inboxRepository.findOne({
      where: { eventId, consumer: CONSUMER_NAME },
    });
    return !!existing;
  }

  private toHandlerPayload(
    envelope: RabbitMQEventEnvelope,
  ): Record<string, unknown> & { eventId: string; eventType: string } {
    const payload =
      envelope.payload && typeof envelope.payload === 'object'
        ? (envelope.payload as Record<string, unknown>)
        : { value: envelope.payload };
    return {
      ...payload,
      eventId: envelope.eventId,
      eventType: envelope.eventType,
    };
  }
}
