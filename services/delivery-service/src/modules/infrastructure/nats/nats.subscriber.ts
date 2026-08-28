import { Injectable, Logger, OnModuleDestroy, OnModuleInit } from '@nestjs/common';

@Injectable()
export class NatsSubscriber implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(NatsSubscriber.name);
  onModuleInit(): void { this.logger.log('Delivery NATS subscriber initialized'); }
  onModuleDestroy(): void { this.logger.log('Delivery NATS subscriber stopped'); }
}
