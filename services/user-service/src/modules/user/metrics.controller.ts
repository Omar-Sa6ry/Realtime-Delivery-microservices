import { Controller, Get, Res } from '@nestjs/common';
import { MetricsService } from '@delivery/common';

@Controller('user')
export class MetricsController {
  constructor(private readonly metricsService: MetricsService) {}

  @Get('metrics')
  async getMetrics(@Res() res: any) {
    res.set('Content-Type', this.metricsService.getContentType());
    const metrics = await this.metricsService.getMetrics();
    res.end(metrics);
  }
}
