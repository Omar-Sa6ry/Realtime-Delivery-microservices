import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { MoreThanOrEqual, Repository } from 'typeorm';
import { Outbox, OutboxStatus } from '../entities/outbox.entity';

@Injectable()
export class OutboxRepository {
  constructor(
    @InjectRepository(Outbox) private readonly repository: Repository<Outbox>,
  ) {}

  createEvent(
    input: Pick<Outbox, 'eventId' | 'eventType' | 'aggregateId' | 'payload'> &
      Partial<Pick<Outbox, 'aggregateType'>>,
  ): Outbox {
    return this.repository.create({
      ...input,
      aggregateType: input.aggregateType ?? 'delivery',
      status: OutboxStatus.PENDING,
      attempts: 0,
      availableAt: new Date(),
      publishedAt: null,
      lastError: null,
    });
  }

  save(event: Outbox): Promise<Outbox> {
    return this.repository.save(event);
  }

  async claimPending(limit: number): Promise<Outbox[]> {
    const events = await this.repository.find({
      where: [
        {
          status: OutboxStatus.PENDING,
          availableAt: MoreThanOrEqual(new Date(0)),
        },
        {
          status: OutboxStatus.FAILED,
          availableAt: MoreThanOrEqual(new Date(0)),
        },
      ],
      order: { createdAt: 'ASC' },
      take: Math.min(Math.max(limit, 1), 100),
    });
    const claimed: Outbox[] = [];
    for (const event of events) {
      const result = await this.repository.update(
        { id: event.id, status: event.status },
        { status: OutboxStatus.PROCESSING, attempts: () => 'attempts + 1' },
      );
      if (result.affected === 1) {
        event.status = OutboxStatus.PROCESSING;
        event.attempts += 1;
        claimed.push(event);
      }
    }
    return claimed;
  }
  
  markPublished(event: Outbox): Promise<Outbox> {
    event.status = OutboxStatus.PUBLISHED;
    event.publishedAt = new Date();
    event.lastError = null;
    return this.repository.save(event);
  }

  markFailed(
    event: Outbox,
    error: Error,
    retryDelayMs: number,
  ): Promise<Outbox> {
    event.status = OutboxStatus.FAILED;
    event.lastError = error.message;
    event.availableAt = new Date(Date.now() + retryDelayMs);
    return this.repository.save(event);
  }
}
