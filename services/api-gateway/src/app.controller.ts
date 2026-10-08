import { Controller, Get } from '@nestjs/common';
import { BooleanResponse } from '@delivery-micro/shard';

@Controller('ping')
export class AppController {
  @Get()
  pingForApiGateway(): BooleanResponse {
    return {
      success: true,
      statusCode: 200,
      message: 'Api Gateway is running',
      data: true,
    };
  }
}
