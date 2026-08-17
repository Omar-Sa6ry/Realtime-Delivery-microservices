export interface LocationUpdatePayload {
  deliveryId: string;
  lat: number;
  lng: number;
  accuracy?: number;
  speed?: number;
  heading?: number;
  timestamp: string;
}

export interface LocationUpdateResult {
  valid: boolean;
  errors?: string[];
  deliveryId?: string;
  lat?: number;
  lng?: number;
  accuracy?: number;
  speed?: number;
  heading?: number;
  timestamp?: number;
}