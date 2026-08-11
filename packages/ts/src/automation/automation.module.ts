import { Module, Global } from '@nestjs/common';
import { HealthService } from './health.service';
import { AlertService } from './alert.service';

@Global()
@Module({
  providers: [HealthService, AlertService],
  exports: [HealthService, AlertService],
})
export class AutomationModule {}
