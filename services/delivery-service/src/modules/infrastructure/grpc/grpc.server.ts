import { Controller, Inject, OnModuleInit } from '@nestjs/common';
import { GrpcMethod, ClientGrpc } from '@nestjs/microservices';

interface ValidateDriverIdRequest {
  driverId: string;
}

interface ValidateDriverIdResponse {
  valid: boolean;
  driverId: string;
  message: string;
}

interface DriverGrpcService {
  ValidateDriverId(request: ValidateDriverIdRequest): Promise<ValidateDriverIdResponse>;
}

@Controller()
export class GrpcServer implements OnModuleInit {
  private driverService: DriverGrpcService;

  constructor(@Inject('DRIVER_SERVICE') private readonly client: ClientGrpc) {}

  onModuleInit() {
    this.driverService = this.client.getService<DriverGrpcService>('DriverService');
  }

  @GrpcMethod('DriverService', 'ValidateDriverId')
  async validateDriverId(
    request: ValidateDriverIdRequest,
  ): Promise<ValidateDriverIdResponse> {
    try {
      return await this.driverService.ValidateDriverId(request);
    } catch {
      return {
        valid: false,
        driverId: request.driverId,
        message: 'Driver validation failed',
      };
    }
  }
}