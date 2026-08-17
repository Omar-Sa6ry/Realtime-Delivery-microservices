import { Global, Module } from '@nestjs/common';
import { EventMapper } from './event.mapper';
import { EventDeduplicator } from './event-deduplicator';
import { IdempotencyStore } from './idempotency.store';

@Global()
@Module({
  providers: [EventMapper, EventDeduplicator, IdempotencyStore],
  exports: [EventMapper, EventDeduplicator, IdempotencyStore],
})
export class EventsModule {}