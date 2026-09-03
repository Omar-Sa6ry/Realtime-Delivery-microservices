import { forwardRef, Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { KafkaModule } from '@delivery/common';
import { DeliveryKafkaConsumer } from './kafka.consumer';
import { DeliveryModule } from '../../delivery/delivery.module';

@Module({
  imports: [
    KafkaModule.registerAsync({
      imports: [ConfigModule],
      inject: [ConfigService],
      useFactory: (config: ConfigService) => ({
        clientId: config.get<string>('KAFKA_CLIENT_ID', 'delivery-service'),
        brokers: (config.get<string>('KAFKA_BROKERS', 'localhost:9092') || '')
          .split(',')
          .map((b) => b.trim())
          .filter(Boolean),
      }),
    }),
    forwardRef(() => DeliveryModule),
  ],
  providers: [DeliveryKafkaConsumer],
  exports: [KafkaModule, DeliveryKafkaConsumer],
})
export class DeliveryKafkaModule {}
