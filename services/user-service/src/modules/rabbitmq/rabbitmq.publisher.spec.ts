import { Test, TestingModule } from '@nestjs/testing';
import { UserRabbitMQPublisher } from './rabbitmq.publisher';
import { RabbitMQService, RabbitMQExchanges, UsersRoutingKeys } from '@delivery-micro/shard';

describe('UserRabbitMQPublisher', () => {
  let service: UserRabbitMQPublisher;
  let rabbitmqService: jest.Mocked<RabbitMQService>;

  beforeEach(async () => {
    rabbitmqService = {
      publish: jest.fn().mockResolvedValue(undefined),
    } as unknown as jest.Mocked<RabbitMQService>;

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        UserRabbitMQPublisher,
        { provide: RabbitMQService, useValue: rabbitmqService },
      ],
    }).compile();

    service = module.get<UserRabbitMQPublisher>(UserRabbitMQPublisher);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('publishUserCreated', () => {
    it('should publish user.created event with correct parameters', async () => {
      const user = {
        id: 'user-123',
        email: 'test@example.com',
        firstName: 'John',
        lastName: 'Doe',
        role: 'USER',
        isActive: true,
        createdAt: new Date('2024-01-01'),
      };

      await service.publishUserCreated(user);

      expect(rabbitmqService.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.USERS,
        UsersRoutingKeys.CREATED,
        'user.created',
        {
          userId: 'user-123',
          email: 'test@example.com',
          firstName: 'John',
          lastName: 'Doe',
          role: 'USER',
          isActive: true,
          createdAt: '2024-01-01T00:00:00.000Z',
        },
        { aggregateId: 'user-123', aggregateType: 'user' },
      );
    });

    it('should handle optional fields with defaults', async () => {
      const user = {
        id: 'user-123',
        email: 'test@example.com',
      };

      await service.publishUserCreated(user);

      expect(rabbitmqService.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.USERS,
        UsersRoutingKeys.CREATED,
        'user.created',
        expect.objectContaining({
          userId: 'user-123',
          email: 'test@example.com',
          firstName: null,
          lastName: null,
          role: 'USER',
          isActive: true,
        }),
        { aggregateId: 'user-123', aggregateType: 'user' },
      );
    });

    it('should handle string createdAt', async () => {
      const user = {
        id: 'user-123',
        email: 'test@example.com',
        createdAt: '2024-01-01T00:00:00.000Z',
      };

      await service.publishUserCreated(user);

      expect(rabbitmqService.publish).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(String),
        expect.any(String),
        expect.objectContaining({
          createdAt: '2024-01-01T00:00:00.000Z',
        }),
        expect.any(Object),
      );
    });

    it('should not throw if publish fails (best-effort)', async () => {
      rabbitmqService.publish.mockRejectedValue(new Error('RMQ connection failed'));

      await expect(
        service.publishUserCreated({ id: 'user-123', email: 'test@example.com' }),
      ).resolves.toBeUndefined();
    });
  });

  describe('publishUserUpdated', () => {
    it('should publish user.updated event with correct parameters', async () => {
      const user = {
        id: 'user-123',
        email: 'test@example.com',
        firstName: 'Jane',
        lastName: 'Smith',
        role: 'ADMIN',
        isActive: true,
        createdAt: new Date('2024-01-01'),
      };

      await service.publishUserUpdated(user);

      expect(rabbitmqService.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.USERS,
        UsersRoutingKeys.UPDATED,
        'user.updated',
        expect.objectContaining({
          userId: 'user-123',
          email: 'test@example.com',
          firstName: 'Jane',
          lastName: 'Smith',
          role: 'ADMIN',
          isActive: true,
          createdAt: '2024-01-01T00:00:00.000Z',
          updatedAt: expect.any(String),
        }),
        { aggregateId: 'user-123', aggregateType: 'user' },
      );
    });

    it('should handle partial updates with only id', async () => {
      const user = { id: 'user-123' };

      await service.publishUserUpdated(user);

      expect(rabbitmqService.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.USERS,
        UsersRoutingKeys.UPDATED,
        'user.updated',
        expect.objectContaining({
          userId: 'user-123',
          email: null,
          firstName: null,
          lastName: null,
          role: null,
          isActive: null,
          createdAt: null,
          updatedAt: expect.any(String),
        }),
        { aggregateId: 'user-123', aggregateType: 'user' },
      );
    });

    it('should not throw if publish fails (best-effort)', async () => {
      rabbitmqService.publish.mockRejectedValue(new Error('RMQ connection failed'));

      await expect(
        service.publishUserUpdated({ id: 'user-123' }),
      ).resolves.toBeUndefined();
    });
  });

  describe('publishUserDeleted', () => {
    it('should publish user.deleted event with correct parameters', async () => {
      await service.publishUserDeleted('user-123');

      expect(rabbitmqService.publish).toHaveBeenCalledWith(
        RabbitMQExchanges.USERS,
        UsersRoutingKeys.ALL,
        'user.deleted',
        {
          userId: 'user-123',
          deletedAt: expect.any(String),
        },
        { aggregateId: 'user-123', aggregateType: 'user' },
      );
    });

    it('should not throw if publish fails (best-effort)', async () => {
      rabbitmqService.publish.mockRejectedValue(new Error('RMQ connection failed'));

      await expect(service.publishUserDeleted('user-123')).resolves.toBeUndefined();
    });
  });
});