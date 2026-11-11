import { SmsRabbitMQConsumer } from './sms.consumer';
import { EventHandlerFactory } from '../../kafka/event-handlers/event-handler.factory';
import { NotificationInbox } from '../../../common/database/entities/notification-inbox.entity';
import { createMockRepository, MockRepository } from '../../../test-utils';
import { RabbitMQEventEnvelope, RabbitMQService } from '@delivery-micro/shard';

describe('SmsRabbitMQConsumer', () => {
  let consumer: SmsRabbitMQConsumer;
  let factory: EventHandlerFactory;
  let inboxRepository: MockRepository<NotificationInbox>;

  const envelope = (
    overrides: Partial<RabbitMQEventEnvelope> = {},
  ): RabbitMQEventEnvelope =>
    ({
      eventId: 'e-s-1',
      eventType: 'payment.failed',
      eventVersion: 1,
      occurredAt: new Date().toISOString(),
      timestamp: Date.now(),
      aggregateType: 'payment',
      aggregateId: 'pay-1',
      producer: 'payment-service',
      payload: { userId: 'u-1' },
      ...overrides,
    }) as RabbitMQEventEnvelope;

  beforeEach(() => {
    factory = new EventHandlerFactory();
    inboxRepository = createMockRepository<NotificationInbox>();
    consumer = new SmsRabbitMQConsumer(
      {} as RabbitMQService,
      factory,
      inboxRepository as any,
    );
  });

  it('uses a prefetch of 20', () => {
    expect((consumer as any).prefetch).toBe(20);
  });

  it('detects duplicates via the shared inbox consumer name', async () => {
    inboxRepository.findOne.mockResolvedValue({ id: 'x' });
    await expect((consumer as any).isDuplicate('e-s-1')).resolves.toBe(true);
  });

  it('routes events and persists the inbox record', async () => {
    const handler = { handle: jest.fn().mockResolvedValue(undefined) };
    factory.registerHandler('payment.failed', handler);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await (consumer as any).handleEvent(envelope(), {});

    expect(handler.handle).toHaveBeenCalledWith(
      expect.objectContaining({
        eventId: 'e-s-1',
        eventType: 'payment.failed',
      }),
    );
    expect(inboxRepository.create).toHaveBeenCalledWith(
      expect.objectContaining({ consumer: 'notification-service-rabbitmq' }),
    );
    expect(inboxRepository.save).toHaveBeenCalled();
  });

  it('wraps primitive payloads', async () => {
    const handler = { handle: jest.fn().mockResolvedValue(undefined) };
    factory.registerHandler('payment.failed', handler);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await (consumer as any).handleEvent(envelope({ payload: 42 as any }), {});

    expect(handler.handle).toHaveBeenCalledWith(
      expect.objectContaining({ value: 42 }),
    );
  });
});
