import { Logger, OnModuleInit, OnModuleDestroy } from '@nestjs/common';
import { Consumer, EachMessagePayload } from 'kafkajs';
import { KafkaService } from './kafka.service';

/**
 * Base Kafka consumer class that handles automated topic creation
 * via Admin client, connection retries, and clean disconnects.
 */
export abstract class BaseKafkaConsumer implements OnModuleInit, OnModuleDestroy {
  protected abstract readonly logger: any;
  protected abstract consumer: Consumer;
  protected abstract kafkaService: KafkaService;
  protected abstract readonly topics: string[];
  protected connected = false;

  async onModuleInit(): Promise<void> {
    await this.startConsumerWithRetry();
  }

  private async startConsumerWithRetry(retries = 10, delayMs = 5000): Promise<void> {
    for (let i = 0; i < retries; i++) {
      try {
        const admin = this.kafkaService.getClient().admin();
        await admin.connect();
        const existingTopics = await admin.listTopics();
        const topicsToCreate = this.topics
          .filter((t) => !existingTopics.includes(t))
          .map((t) => ({ topic: t }));

        if (topicsToCreate.length > 0) {
          await admin.createTopics({ topics: topicsToCreate });
          this.logger.log(`Created missing Kafka topics: ${topicsToCreate.map((t) => t.topic).join(', ')}`);
        }
        await admin.disconnect();

        await this.consumer.connect();
        for (const topic of this.topics) {
          await this.consumer.subscribe({ topic, fromBeginning: false });
        }
        await this.consumer.run({
          eachMessage: async (payload) => this.handleMessage(payload),
        });
        
        this.connected = true;
        this.logger.log('Kafka consumer started successfully');
        return;
      } catch (err) {
        this.connected = false;
        this.logger.warn(`Kafka consumer failed to start (attempt ${i + 1}/${retries}): ${(err as Error).message}`);
        await new Promise((resolve) => setTimeout(resolve, delayMs));
      }
    }
    this.logger.error('Kafka consumer failed to start after maximum retries');
  }

  protected abstract handleMessage(payload: EachMessagePayload): Promise<void>;

  isConnected(): boolean {
    return this.connected;
  }

  async onModuleDestroy(): Promise<void> {
    try {
      await this.consumer.disconnect();
      this.connected = false;
      this.logger.log('Kafka consumer disconnected');
    } catch (err) {
      this.logger.error(`Kafka disconnect failed: ${(err as Error).message}`);
    }
  }
}
