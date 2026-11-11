import { PushRabbitMQConsumer } from './push.consumer';
import { EventHandlerFactory } from '../../kafka/event-handlers/event-handler.factory';
import { NotificationInbox } from '../../../common/database/entities/notification-inbox.entity';
import { createMockRepository, MockRepository } from '../../../test-utils';
import { RabbitMQEventEnvelope, RabbitMQService } from '@delivery-micro/shard';

describe('PushRabbitMQConsumer', () => {
  let consumer: PushRabbitMQConsumer;
  let factory: EventHandlerFactory;
  let inboxRepository: MockRepository<NotificationInbox>;

  const envelope = (
    overrides: Partial<RabbitMQEventEnvelope> = {},
  ): RabbitMQEventEnvelope =>
    ({
      eventId: 'e-p-1',
      eventType: 'delivery.completed',
      eventVersion: 1,
      occurredAt: new Date().toISOString(),
      timestamp: Date.now(),
      aggregateType: 'delivery',
      aggregateId: 'del-1',
      producer: 'delivery-service',
      payload: { userId: 'u-1' },
      ...overrides,
    }) as RabbitMQEventEnvelope;

  beforeEach(() => {
    factory = new EventHandlerFactory();
    inboxRepository = createMockRepository<NotificationInbox>();
    consumer = new PushRabbitMQConsumer(
      {} as RabbitMQService,
      factory,
      inboxRepository as any,
    );
  });

  it('uses a prefetch of 50', () => {
    expect((consumer as any).prefetch).toBe(50);
  });

  it('detects duplicates via the shared inbox consumer name', async () => {
    inboxRepository.findOne.mockResolvedValue({ id: 'x' });
    await expect((consumer as any).isDuplicate('e-p-1')).resolves.toBe(true);
  });

  it('routes events and persists the inbox record', async () => {
    const handler = { handle: jest.fn().mockResolvedValue(undefined) };
    factory.registerHandler('delivery.completed', handler);
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await (consumer as any).handleEvent(envelope(), {});

    expect(handler.handle).toHaveBeenCalledWith(
      expect.objectContaining({ eventId: 'e-p-1' }),
    );
    expect(inboxRepository.save).toHaveBeenCalled();
  });

  it('skips handler invocation for unknown types but records the inbox', async () => {
    inboxRepository.save.mockImplementation(async (r: any) => r);

    await (consumer as any).handleEvent(envelope({ eventType: 'nope' }), {});

    expect(inboxRepository.save).toHaveBeenCalled();
  });
});
