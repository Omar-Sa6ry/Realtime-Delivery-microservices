import { Module } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { KafkaModule } from '@delivery/common';

@Module({
  imports: [KafkaModule.registerAsync({
    imports: [ConfigModule],
    inject: [ConfigService],
    useFactory: (config: ConfigService) => ({
      clientId: config.get<string>('KAFKA_CLIENT_ID', 'delivery-service'),
      brokers: (config.get<string>('KAFKA_BROKERS', 'localhost:9092') || '').split(',').map((b) => b.trim()).filter(Boolean),
    }),
  })],
  exports: [KafkaModule],
})
export class DeliveryKafkaModule {}
