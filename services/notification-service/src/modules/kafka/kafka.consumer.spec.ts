import { KafkaConsumer } from './kafka.consumer';
import { EventHandlerFactory } from './event-handlers/event-handler.factory';
import { NotificationInbox } from '../../common/database/entities/notification-inbox.entity';
import { createMockRepository, MockRepository } from '../../test-utils';
import { ConfigService } from '@nestjs/config';
import { EachMessagePayload } from 'kafkajs';

describe('KafkaConsumer', () => {
  let consumer: KafkaConsumer;
  let inboxRepository: MockRepository<NotificationInbox>;
  let factory: EventHandlerFactory;
  let kafkaService: { consumer: jest.Mock };
  let configService: { get: jest.Mock };

  const messagePayload = (
    body: Record<string, unknown> | null,
    topic = 'delivery.created',
  ): EachMessagePayload =>
    ({
      topic,
      partition: 0,
      message: {
        value: body === null ? null : Buffer.from(JSON.stringify(body)),
      },
    }) as unknown as EachMessagePayload;

  beforeEach(() => {
    inboxRepository = createMockRepository<NotificationInbox>();
    factory = new EventHandlerFactory();
    kafkaService = { consumer: jest.fn().mockReturnValue({}) };
    configService = { get: jest.fn().mockReturnValue('notification-service') };

    consumer = new KafkaConsumer(
      configService as unknown as ConfigService,
      inboxRepository as any,
      factory,
      kafkaService as any,
    );
  });

  it('subscribes using the configured consumer group', () => {
    expect(configService.get).toHaveBeenCalledWith(
      'KAFKA_GROUP_ID',
      'notification-service',
    );
    expect(kafkaService.consumer).toHaveBeenCalledWith('notification-service');
  });

  it('ignores messages with a null value', async () => {
    await consumer.handleMessage(messagePayload(null));

    expect(inboxRepository.findOne).not.toHaveBeenCalled();
    expect(inboxRepository.save).not.toHaveBeenCalled();
  });

  it('ignores messages without an eventId', async () => {
    await consumer.handleMessage(
      messagePayload({ eventType: 'delivery.created', userId: 'u-1' }),
    );

    expect(inboxRepository.findOne).not.toHaveBeenCalled();
    expect(inboxRepository.save).not.toHaveBeenCalled();
  });

  it('skips events that were already processed (inbox idempotency)', async () => {
    inboxRepository.findOne.mockResolvedValue({ id: 'inbox-1' });

    await consumer.handleMessage(
      messagePayload({
        eventId: 'e-1',
        eventType: 'delivery.created',
        userId: 'u-1',
      }),
    );

    expect(inboxRepository.findOne).toHaveBeenCalledWith({
      where: { eventId: 'e-1', consumer: 'notification-service' },
    });
    expect(inboxRepository.save).not.toHaveBeenCalled();
  });

  it('uses payload.id when eventId is missing but still processes', async () => {
    const handler = { handle: jest.fn() };
    factory.registerHandler('delivery.created', handler);
    inboxRepository.findOne.mockResolvedValue(null);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await consumer.handleMessage(
      messagePayload({
        id: 'legacy-1',
        eventType: 'delivery.created',
        userId: 'u-1',
      }),
    );

    expect(handler.handle).toHaveBeenCalled();
    expect(inboxRepository.create).toHaveBeenCalledWith(
      expect.objectContaining({ eventId: 'legacy-1' }),
    );
  });

  it('falls back to the topic name when eventType is missing', async () => {
    inboxRepository.findOne.mockResolvedValue(null);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await consumer.handleMessage(
      messagePayload({ eventId: 'e-1', userId: 'u-1' }, 'payment.completed'),
    );

    expect(inboxRepository.create).toHaveBeenCalledWith(
      expect.objectContaining({ eventType: 'payment.completed' }),
    );
  });

  it('dispatches to the matching handler and records the inbox row', async () => {
    const handler = { handle: jest.fn().mockResolvedValue(undefined) };
    factory.registerHandler('delivery.created', handler);
    inboxRepository.findOne.mockResolvedValue(null);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    const payload = {
      eventId: 'e-1',
      eventType: 'delivery.created',
      userId: 'u-1',
    };
    await consumer.handleMessage(messagePayload(payload));

    expect(handler.handle).toHaveBeenCalledWith(payload);
    expect(inboxRepository.create).toHaveBeenCalledWith(
      expect.objectContaining({
        eventId: 'e-1',
        eventType: 'delivery.created',
        consumer: 'notification-service',
        processedAt: expect.any(Date),
      }),
    );
    expect(inboxRepository.save).toHaveBeenCalled();
  });

  it('still records the inbox row when no handler is registered', async () => {
    inboxRepository.findOne.mockResolvedValue(null);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await consumer.handleMessage(
      messagePayload({
        eventId: 'e-2',
        eventType: 'unknown.event',
        userId: 'u-1',
      }),
    );

    expect(inboxRepository.save).toHaveBeenCalled();
  });

  it('swallows handler errors without crashing the consumer', async () => {
    const handler = {
      handle: jest.fn().mockRejectedValue(new Error('handler boom')),
    };
    factory.registerHandler('delivery.created', handler);
    inboxRepository.findOne.mockResolvedValue(null);

    await expect(
      consumer.handleMessage(
        messagePayload({
          eventId: 'e-3',
          eventType: 'delivery.created',
          userId: 'u-1',
        }),
      ),
    ).resolves.toBeUndefined();
    expect(inboxRepository.save).not.toHaveBeenCalled();
  });
});
