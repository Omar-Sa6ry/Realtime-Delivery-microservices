import { Controller, Get } from '@nestjs/common';
import { HealthService } from '@delivery/common';

@Controller('health')
export class HealthController {
  constructor(private readonly healthService: HealthService) {}

  @Get()
  checkHealth() {
    return {
      status: 'ok',
      service: 'api-gateway',
      timestamp: new Date().toISOString(),
      system: this.healthService.getSystemStats(),
    };
  }
}
