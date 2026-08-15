import { Injectable, Logger } from '@nestjs/common';
import { IEventHandler } from './event-handler.interface';

@Injectable()
export class EventHandlerFactory {
  private handlers = new Map<string, IEventHandler>();
  private readonly logger = new Logger(EventHandlerFactory.name);

  registerHandler(eventType: string, handler: IEventHandler) {
    this.handlers.set(eventType, handler);
    this.logger.debug(`Registered handler for event: ${eventType}`);
  }

  getHandler(eventType: string): IEventHandler | undefined {
    return this.handlers.get(eventType);
  }
}
