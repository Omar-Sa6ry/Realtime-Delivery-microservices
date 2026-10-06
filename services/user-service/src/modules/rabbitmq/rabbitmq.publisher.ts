import { Injectable, Logger } from '@nestjs/common';
import {
  RabbitMQExchanges,
  RabbitMQService,
  UsersRoutingKeys,
} from '@delivery-micro/shard';

@Injectable()
export class UserRabbitMQPublisher {
  private readonly logger = new Logger(UserRabbitMQPublisher.name);

  constructor(private readonly rabbitmq: RabbitMQService) {}

  async publishUserCreated(user: {
    id: string;
    email: string;
    firstName?: string;
    lastName?: string;
    role?: string;
    isActive?: boolean;
    createdAt?: Date | string;
  }): Promise<void> {
    await this.rabbitmqPublish(
      RabbitMQExchanges.USERS,
      UsersRoutingKeys.CREATED,
      'user.created',
      {
        userId: user.id,
        email: user.email,
        firstName: user.firstName ?? null,
        lastName: user.lastName ?? null,
        role: user.role ?? 'USER',
        isActive: user.isActive ?? true,
        createdAt:
          user.createdAt instanceof Date
            ? user.createdAt.toISOString()
            : (user.createdAt ?? new Date().toISOString()),
      },
      { aggregateId: user.id, aggregateType: 'user' },
    );
  }

  async publishUserUpdated(user: {
    id: string;
    email?: string;
    firstName?: string;
    lastName?: string;
    role?: string;
    isActive?: boolean;
    createdAt?: Date | string;
  }): Promise<void> {
    await this.rabbitmqPublish(
      RabbitMQExchanges.USERS,
      UsersRoutingKeys.UPDATED,
      'user.updated',
      {
        userId: user.id,
        email: user.email ?? null,
        firstName: user.firstName ?? null,
        lastName: user.lastName ?? null,
        role: user.role ?? null,
        isActive: user.isActive ?? null,
        createdAt:
          user.createdAt instanceof Date
            ? user.createdAt.toISOString()
            : (user.createdAt ?? null),
        updatedAt: new Date().toISOString(),
      },
      { aggregateId: user.id, aggregateType: 'user' },
    );
  }

  async publishUserDeleted(userId: string): Promise<void> {
    await this.rabbitmqPublish(
      RabbitMQExchanges.USERS,
      UsersRoutingKeys.ALL,
      'user.deleted',
      {
        userId,
        deletedAt: new Date().toISOString(),
      },
      { aggregateId: userId, aggregateType: 'user' },
    );
  }

  private async rabbitmqPublish(
    exchange: string,
    routingKey: string,
    eventType: string,
    payload: Record<string, unknown>,
    options?: { aggregateId?: string; aggregateType?: string },
  ): Promise<void> {
    try {
      await this.rabbitmq.publish(
        exchange,
        routingKey,
        eventType,
        payload,
        options,
      );
    } catch (err) {
      // Best-effort: log but do not break the user mutation.
      this.logger.warn(
        `UserRabbitMQPublisher: best-effort publish failed [${exchange}/${routingKey}]: ${(err as Error).message}`,
      );
    }
  }
}
