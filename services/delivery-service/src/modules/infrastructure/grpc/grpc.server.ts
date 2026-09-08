import { Controller, Inject, OnModuleInit } from '@nestjs/common';
import type { ClientGrpc } from '@nestjs/microservices';
import { firstValueFrom } from 'rxjs';
import type {
  DriverGrpcService,
  GetDriverResponse,
  GetDriverStatusResponse,
  ValidateIDResponse,
} from './driver-grpc.types';

@Controller()
export class GrpcServer implements OnModuleInit {
  private driverService: DriverGrpcService;

  constructor(@Inject('DRIVER_SERVICE') private readonly client: ClientGrpc) {}

  onModuleInit() {
    this.driverService = this.client.getService<DriverGrpcService>('DriverService');
  }

  async validateDriverId(driverId: string): Promise<ValidateIDResponse> {
    try {
      return await firstValueFrom(
        this.driverService.ValidateID({ id: driverId, idType: 'driver' }),
      );
    } catch {
      return { valid: false, driverId, status: '', message: 'Driver validation failed' };
    }
  }

  async validateUserId(userId: string): Promise<ValidateIDResponse> {
    try {
      return await firstValueFrom(
        this.driverService.ValidateID({ id: userId, idType: 'user' }),
      );
    } catch {
      return { valid: false, driverId: '', status: '', message: 'User validation failed' };
    }
  }

  async getDriver(driverId: string): Promise<GetDriverResponse> {
    try {
      return await firstValueFrom(
        this.driverService.GetDriver({ driverId }),
      );
    } catch {
      return { found: false, driverId, userId: '', status: '', vehicleType: 'VEHICLE_TYPE_UNKNOWN' };
    }
  }

  async getDriverStatus(driverId: string): Promise<GetDriverStatusResponse> {
    try {
      return await firstValueFrom(
        this.driverService.GetDriverStatus({ driverId }),
      );
    } catch {
      return { driverId, status: 'UNKNOWN', hasActiveAssignment: false, activeDeliveryId: '' };
    }
  }
}