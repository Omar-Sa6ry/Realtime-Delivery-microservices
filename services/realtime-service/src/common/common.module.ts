import { Global, Module } from '@nestjs/common';
import { RedisModule, RedisService } from '@bts-soft/cache';
import { HealthService, MetricsModule } from '@delivery/common';
import { TranslationModule } from './translation/translation.module';
import { HealthController } from './health.controller';
import { RealtimeMetricsService } from './metrics/realtime-metrics.service';
import { NatsModule } from '../modules/nats/nats.module';
import { KafkaConsumerModule } from '../modules/kafka/kafka.module';

@Global()
@Module({
  imports: [
    RedisModule,
    TranslationModule,
    MetricsModule,
    NatsModule,
    KafkaConsumerModule,
  ],
  controllers: [HealthController],
  providers: [
    HealthService,
    RealtimeMetricsService,
    { provide: 'SHARED_REDIS_SERVICE', useExisting: RedisService },
  ],
  exports: [
    RedisModule,
    TranslationModule,
    HealthService,
    RealtimeMetricsService,
    'SHARED_REDIS_SERVICE',
  ],
})
export class CommonModule {}