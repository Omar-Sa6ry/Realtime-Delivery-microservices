import { EventHandlerFactory } from './event-handler.factory';
import { IEventHandler } from './event-handler.interface';

describe('EventHandlerFactory', () => {
  let factory: EventHandlerFactory;

  beforeEach(() => {
    factory = new EventHandlerFactory();
  });

  it('registers and returns handlers by event type', () => {
    const handler: IEventHandler = { handle: jest.fn() };

    factory.registerHandler('delivery.created', handler);

    expect(factory.getHandler('delivery.created')).toBe(handler);
  });

  it('returns undefined for unknown event types', () => {
    expect(factory.getHandler('unknown.event')).toBeUndefined();
  });

  it('overwrites previously registered handlers', () => {
    const first: IEventHandler = { handle: jest.fn() };
    const second: IEventHandler = { handle: jest.fn() };

    factory.registerHandler('payment.failed', first);
    factory.registerHandler('payment.failed', second);

    expect(factory.getHandler('payment.failed')).toBe(second);
  });
});
