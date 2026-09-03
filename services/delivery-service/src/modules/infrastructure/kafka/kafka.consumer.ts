import {
  Injectable,
  Logger,
  OnModuleDestroy,
  OnModuleInit,
} from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Consumer, EachMessagePayload } from 'kafkajs';
import {
  KafkaService,
  KafkaEventEnvelope,
  PaymentKafkaTopics,
  PaymentCompletedPayload,
  PaymentFailedPayload,
} from '@delivery/common';
import { DeliveryCommandService } from '../../delivery/services/delivery-command.service';
import { PaymentStatus } from '../../delivery/enums/payment-status.enum';

@Injectable()
export class DeliveryKafkaConsumer implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(DeliveryKafkaConsumer.name);
  private consumer: Consumer;
  private connected = false;

  constructor(
    private readonly kafka: KafkaService,
    private readonly config: ConfigService,
    private readonly commands: DeliveryCommandService,
  ) {
    this.consumer = this.kafka.consumer(
      this.config.get<string>('KAFKA_GROUP_ID', 'delivery-service-group'),
    );
  }

  async onModuleInit(): Promise<void> {
    try {
      await this.consumer.connect();
      await this.consumer.subscribe({
        topics: [
          PaymentKafkaTopics.PAYMENT_COMPLETED,
          PaymentKafkaTopics.PAYMENT_FAILED,
        ],
        fromBeginning: false,
      });
      await this.consumer.run({
        eachMessage: async (payload) => this.handleMessage(payload),
      });
      this.connected = true;
      this.logger.log('Delivery Kafka consumer started (delivery-service-group)');
    } catch (err: any) {
      this.connected = false;
      this.logger.error(`Delivery Kafka consumer failed to start: ${err?.message}`);
    }
  }

  private async handleMessage(payload: EachMessagePayload): Promise<void> {
    try {
      const raw = payload.message.value?.toString() || '{}';
      const envelope = JSON.parse(raw) as KafkaEventEnvelope;
      if (!envelope?.eventType) {
        this.logger.warn('Dropping malformed Kafka message');
        return;
      }

      switch (envelope.eventType) {
        case PaymentKafkaTopics.PAYMENT_COMPLETED: {
          const data = envelope.payload as PaymentCompletedPayload;
          if (data?.deliveryId) {
            this.logger.log(`Payment completed for delivery: ${data.deliveryId}`);
            await this.commands.updatePaymentStatus(
              data.deliveryId,
              PaymentStatus.COMPLETED,
            );
          }
          break;
        }

        case PaymentKafkaTopics.PAYMENT_FAILED: {
          const data = envelope.payload as PaymentFailedPayload;
          if (data?.deliveryId) {
            this.logger.warn(`Payment failed for delivery: ${data.deliveryId}`);
            await this.commands.updatePaymentStatus(
              data.deliveryId,
              PaymentStatus.FAILED,
            );
            await this.commands.cancel(
              data.deliveryId,
              'system',
              `Payment failed: ${data.reason ?? 'Unknown error'}`,
            );
          }
          break;
        }

        default:
          this.logger.debug(`Unhandled Kafka event: ${envelope.eventType}`);
      }
    } catch (err: any) {
      this.logger.error(`Error processing Kafka message: ${err?.message}`);
    }
  }

  async onModuleDestroy(): Promise<void> {
    if (this.connected) {
      try {
        await this.consumer.disconnect();
        this.connected = false;
      } catch (err: any) {
        this.logger.error(`Kafka disconnect error: ${err?.message}`);
      }
    }
  }
}
