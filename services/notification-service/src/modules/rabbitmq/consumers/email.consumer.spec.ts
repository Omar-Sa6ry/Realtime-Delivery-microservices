import { EmailRabbitMQConsumer } from './email.consumer';
import { EventHandlerFactory } from '../../kafka/event-handlers/event-handler.factory';
import { NotificationInbox } from '../../../common/database/entities/notification-inbox.entity';
import { createMockRepository, MockRepository } from '../../../test-utils';
import { RabbitMQEventEnvelope, RabbitMQService } from '@delivery-micro/shard';

describe('EmailRabbitMQConsumer', () => {
  let consumer: EmailRabbitMQConsumer;
  let factory: EventHandlerFactory;
  let inboxRepository: MockRepository<NotificationInbox>;
  let rabbitmq: any;

  const envelope = (
    overrides: Partial<RabbitMQEventEnvelope> = {},
  ): RabbitMQEventEnvelope =>
    ({
      eventId: 'e-1',
      eventType: 'delivery.created',
      eventVersion: 1,
      occurredAt: new Date().toISOString(),
      timestamp: Date.now(),
      aggregateType: 'delivery',
      aggregateId: 'del-1',
      producer: 'delivery-service',
      payload: { deliveryId: 'del-1', userId: 'u-1' },
      ...overrides,
    }) as RabbitMQEventEnvelope;

  beforeEach(() => {
    factory = new EventHandlerFactory();
    inboxRepository = createMockRepository<NotificationInbox>();
    rabbitmq = {};
    consumer = new EmailRabbitMQConsumer(
      rabbitmq as RabbitMQService,
      factory,
      inboxRepository as any,
    );
  });

  it('uses the notifications email queue and exchange', () => {
    expect((consumer as any).queue).toBeDefined();
    expect((consumer as any).exchange).toBeDefined();
    expect((consumer as any).prefetch).toBe(10);
  });

  it('detects duplicates through the inbox', async () => {
    inboxRepository.findOne.mockResolvedValue({ id: 'inbox-1' });

    await expect((consumer as any).isDuplicate('e-1')).resolves.toBe(true);
    expect(inboxRepository.findOne).toHaveBeenCalledWith({
      where: { eventId: 'e-1', consumer: 'notification-service-rabbitmq' },
    });
  });

  it('returns false when the event has not been seen', async () => {
    inboxRepository.findOne.mockResolvedValue(null);

    await expect((consumer as any).isDuplicate('e-1')).resolves.toBe(false);
  });

  it('routes known events to the handler and stores the inbox row', async () => {
    const handler = { handle: jest.fn().mockResolvedValue(undefined) };
    factory.registerHandler('delivery.created', handler);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await (consumer as any).handleEvent(envelope(), {});

    expect(handler.handle).toHaveBeenCalledWith(
      expect.objectContaining({
        eventId: 'e-1',
        eventType: 'delivery.created',
        deliveryId: 'del-1',
      }),
    );
    expect(inboxRepository.create).toHaveBeenCalledWith(
      expect.objectContaining({
        eventId: 'e-1',
        eventType: 'delivery.created',
        consumer: 'notification-service-rabbitmq',
        processedAt: expect.any(Date),
      }),
    );
  });

  it('still stores the inbox row for unknown event types', async () => {
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await (consumer as any).handleEvent(envelope({ eventType: 'x.y' }), {});

    expect(inboxRepository.save).toHaveBeenCalled();
  });

  it('wraps non-object payloads before invoking the handler', async () => {
    const handler = { handle: jest.fn().mockResolvedValue(undefined) };
    factory.registerHandler('delivery.created', handler);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await (consumer as any).handleEvent(
      envelope({ payload: 'raw-string' as any }),
      {},
    );

    expect(handler.handle).toHaveBeenCalledWith(
      expect.objectContaining({ value: 'raw-string' }),
    );
  });
});
