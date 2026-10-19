import { Test, TestingModule } from '@nestjs/testing';
import { Repository } from 'typeorm';
import { MoreThanOrEqual } from 'typeorm';
import { OutboxRepository } from './outbox.repository';
import { Outbox, OutboxStatus } from '../entities/outbox.entity';
import { mockOutbox } from '../../../../test-utils';

describe('OutboxRepository', () => {
  let repository: OutboxRepository;
  let ormRepository: Partial<Record<keyof Repository<Outbox>, jest.Mock>>;

  beforeEach(async () => {
    ormRepository = {
      create: jest.fn(),
      save: jest.fn(),
      find: jest.fn(),
      update: jest.fn(),
    };

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        OutboxRepository,
        { provide: 'OutboxRepository', useValue: ormRepository },
      ],
    }).compile();

    repository = module.get<OutboxRepository>(OutboxRepository);
  });

  it('should be defined', () => {
    expect(repository).toBeDefined();
  });

  describe('createEvent', () => {
    it('should create outbox event with default values', () => {
      const input = {
        eventId: 'event-123',
        eventType: 'delivery.created',
        aggregateId: 'delivery-123',
        payload: { deliveryId: 'delivery-123' },
      };

      const expectedEvent = {
        ...input,
        aggregateType: 'delivery',
        status: OutboxStatus.PENDING,
        attempts: 0,
        availableAt: expect.any(Date),
        publishedAt: null,
        lastError: null,
      };

      const event = repository.createEvent(input);

      expect(ormRepository.create).toHaveBeenCalledWith(expectedEvent);
    });

    it('should use provided aggregateType', () => {
      const input = {
        eventId: 'event-123',
        eventType: 'delivery.created',
        aggregateId: 'delivery-123',
        payload: { deliveryId: 'delivery-123' },
        aggregateType: 'custom-type',
      };

      repository.createEvent(input);

      expect(ormRepository.create).toHaveBeenCalledWith(expect.objectContaining({
        aggregateType: 'custom-type',
      }));
    });
  });

  describe('save', () => {
    it('should save the outbox event', async () => {
      const event = mockOutbox();
      ormRepository.save.mockResolvedValue(event);

      const result = await repository.save(event);

      expect(ormRepository.save).toHaveBeenCalledWith(event);
      expect(result).toEqual(event);
    });
  });

  describe('claimPending', () => {
    it('should claim pending events', async () => {
      const events = [mockOutbox({ id: '1' }), mockOutbox({ id: '2' })];
      ormRepository.find.mockResolvedValue(events);
      ormRepository.update.mockResolvedValue({ affected: 1 });

      const result = await repository.claimPending(10);

      expect(ormRepository.find).toHaveBeenCalledWith({
        where: [
          { status: OutboxStatus.PENDING, availableAt: MoreThanOrEqual(expect.any(Date)) },
          { status: OutboxStatus.FAILED, availableAt: MoreThanOrEqual(expect.any(Date)) },
        ],
        order: { createdAt: 'ASC' },
        take: 10,
      });
      expect(ormRepository.update).toHaveBeenCalledTimes(2);
      expect(result).toHaveLength(2);
      expect(result[0].status).toBe(OutboxStatus.PROCESSING);
      expect(result[0].attempts).toBe(1);
    });

    it('should not claim events when update affects 0 rows', async () => {
      const events = [mockOutbox({ id: '1' })];
      ormRepository.find.mockResolvedValue(events);
      ormRepository.update.mockResolvedValue({ affected: 0 });

      const result = await repository.claimPending(10);

      expect(result).toHaveLength(0);
    });

    it('should respect batch size limits', async () => {
      ormRepository.find.mockResolvedValue([]);
      ormRepository.update.mockResolvedValue({ affected: 0 });

      await repository.claimPending(200);

      expect(ormRepository.find).toHaveBeenCalledWith(expect.objectContaining({ take: 100 }));

      await repository.claimPending(0);

      expect(ormRepository.find).toHaveBeenCalledWith(expect.objectContaining({ take: 1 }));
    });

    it('should only return successfully claimed events', async () => {
      const events = [mockOutbox({ id: '1' }), mockOutbox({ id: '2' })];
      ormRepository.find.mockResolvedValue(events);
      ormRepository.update
        .mockResolvedValueOnce({ affected: 1 })
        .mockResolvedValueOnce({ affected: 0 });

      const result = await repository.claimPending(10);

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('1');
    });
  });

  describe('markPublished', () => {
    it('should mark event as published', async () => {
      const event = mockOutbox({ status: OutboxStatus.PROCESSING });
      const savedEvent = { ...event, status: OutboxStatus.PUBLISHED, publishedAt: expect.any(Date), lastError: null };
      ormRepository.save.mockResolvedValue(savedEvent);

      const result = await repository.markPublished(event);

      expect(event.status).toBe(OutboxStatus.PUBLISHED);
      expect(event.publishedAt).toBeInstanceOf(Date);
      expect(event.lastError).toBeNull();
      expect(ormRepository.save).toHaveBeenCalledWith(event);
      expect(result).toEqual(savedEvent);
    });
  });

  describe('markFailed', () => {
    it('should mark event as failed with retry delay', async () => {
      const event = mockOutbox({ status: OutboxStatus.PROCESSING, attempts: 1 });
      const error = new Error('Kafka connection failed');
      const retryDelayMs = 5000;
      const savedEvent = { ...event, status: OutboxStatus.FAILED, lastError: error.message, availableAt: expect.any(Date) };
      ormRepository.save.mockResolvedValue(savedEvent);

      const result = await repository.markFailed(event, error, retryDelayMs);

      expect(event.status).toBe(OutboxStatus.FAILED);
      expect(event.lastError).toBe(error.message);
      expect(event.availableAt).toBeInstanceOf(Date);
      expect(event.availableAt.getTime()).toBeGreaterThanOrEqual(Date.now() + retryDelayMs - 100);
      expect(ormRepository.save).toHaveBeenCalledWith(event);
      expect(result).toEqual(savedEvent);
    });

    it('should handle string error', async () => {
      const event = mockOutbox({ status: OutboxStatus.PROCESSING });
      const error = 'String error';
      ormRepository.save.mockResolvedValue(event);

      await repository.markFailed(event, new Error(error), 1000);

      expect(event.lastError).toBe(error);
    });
  });
});