export interface ValidateDriverIdDto {
  driverId: string;
}

export interface ValidateDriverIdResponse {
  valid: boolean;
  driverId: string;
  message: string;
}