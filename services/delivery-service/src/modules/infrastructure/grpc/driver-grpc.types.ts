export interface ValidateIDRequest {
  id: string;
  idType: 'driver' | 'user';
  context?: string;
}

export interface ValidateIDResponse {
  valid: boolean;
  driverId: string;
  status: string;
  message: string;
}

export interface GetDriverRequest {
  driverId: string;
}

export interface GetDriverResponse {
  found: boolean;
  driverId: string;
  userId: string;
  status: string;
  vehicleType: 'CAR' | 'MOTORCYCLE' | 'TRUCK' | 'BICYCLE' | 'VAN' | 'VEHICLE_TYPE_UNKNOWN';
}

export interface GetDriverStatusRequest {
  driverId: string;
}

export interface GetDriverStatusResponse {
  driverId: string;
  status: string;
  hasActiveAssignment: boolean;
  activeDeliveryId: string;
}

export interface DriverGrpcService {
  ValidateID(request: ValidateIDRequest): import('rxjs').Observable<ValidateIDResponse>;
  GetDriver(request: GetDriverRequest): import('rxjs').Observable<GetDriverResponse>;
  GetDriverStatus(request: GetDriverStatusRequest): import('rxjs').Observable<GetDriverStatusResponse>;
}
