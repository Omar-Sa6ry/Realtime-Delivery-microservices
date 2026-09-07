import { Controller, Inject, OnModuleInit } from '@nestjs/common';
import type { ClientGrpc } from '@nestjs/microservices';
import { firstValueFrom, Observable } from 'rxjs';

interface ValidateIDRequest {
  id: string;
  idType: 'driver' | 'user';
  context?: string;
}

interface ValidateIDResponse {
  valid: boolean;
  driverId: string;
  status: string;
  message: string;
}

interface GetDriverRequest {
  driverId: string;
}

interface GetDriverResponse {
  found: boolean;
  driverId: string;
  userId: string;
  status: string;
  vehicleType: string;
}

interface GetDriverStatusRequest {
  driverId: string;
}

interface GetDriverStatusResponse {
  driverId: string;
  status: string;
  hasActiveAssignment: boolean;
  activeDeliveryId: string;
}


interface DriverGrpcService {
  ValidateID(request: ValidateIDRequest): Observable<ValidateIDResponse>;
  GetDriver(request: GetDriverRequest): Observable<GetDriverResponse>;
  GetDriverStatus(request: GetDriverStatusRequest): Observable<GetDriverStatusResponse>;
}

// ─── Controller ────────────────────────────────────────────────────────────

@Controller()
export class GrpcServer implements OnModuleInit {
  private driverService: DriverGrpcService;

  constructor(@Inject('DRIVER_SERVICE') private readonly client: ClientGrpc) {}

  onModuleInit() {
    this.driverService = this.client.getService<DriverGrpcService>('DriverService');
  }


  async validateDriverId(driverId: string): Promise<ValidateIDResponse> {
    try {
      const result = await firstValueFrom(
        this.driverService.ValidateID({ id: driverId, idType: 'driver' }),
      );
      return result;
    } catch {
      return { valid: false, driverId, status: '', message: 'Driver validation failed' };
    }
  }


  async validateUserId(userId: string): Promise<ValidateIDResponse> {
    try {
      const result = await firstValueFrom(
        this.driverService.ValidateID({ id: userId, idType: 'user' }),
      );
      return result;
    } catch {
      return { valid: false, driverId: '', status: '', message: 'User validation failed' };
    }
  }


  async getDriver(driverId: string): Promise<GetDriverResponse> {
    try {
      const result = await firstValueFrom(
        this.driverService.GetDriver({ driverId }),
      );
      return result;
    } catch {
      return { found: false, driverId, userId: '', status: '', vehicleType: '' };
    }
  }

  async getDriverStatus(driverId: string): Promise<GetDriverStatusResponse> {
    try {
      const result = await firstValueFrom(
        this.driverService.GetDriverStatus({ driverId }),
      );
      return result;
    } catch {
      return { driverId, status: 'UNKNOWN', hasActiveAssignment: false, activeDeliveryId: '' };
    }
  }
}