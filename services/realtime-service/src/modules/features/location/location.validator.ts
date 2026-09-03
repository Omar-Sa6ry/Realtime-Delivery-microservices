import { Injectable } from '@nestjs/common';
import { LocationUpdatePayload, LocationUpdateResult } from './location.types';
import { TIMINGS } from '../../../common/common-types/constants';

const ID_RE = /^[0-9a-zA-Z_-]{8,64}$/;

@Injectable()
export class LocationValidator {
  validate(payload: LocationUpdatePayload): LocationUpdateResult {
    const errors: string[] = [];

    const deliveryId = payload?.deliveryId;
    if (typeof deliveryId !== 'string' || !ID_RE.test(deliveryId)) {
      errors.push('deliveryId must be a valid ID');
    }

    const lat = payload?.lat;
    const lng = payload?.lng;
    if (
      typeof lat !== 'number' ||
      !Number.isFinite(lat) ||
      lat < -90 ||
      lat > 90
    ) {
      errors.push('lat must be a finite number between -90 and 90');
    }
    if (
      typeof lng !== 'number' ||
      !Number.isFinite(lng) ||
      lng < -180 ||
      lng > 180
    ) {
      errors.push('lng must be a finite number between -180 and 180');
    }

    for (const field of ['accuracy', 'speed', 'heading'] as const) {
      const value = payload?.[field];
      if (
        value !== undefined &&
        (typeof value !== 'number' || !Number.isFinite(value) || value < 0)
      ) {
        errors.push(`${field} must be a non-negative finite number`);
      }
    }

    let timestamp: number | undefined;
    const rawTs = payload?.timestamp;
    if (typeof rawTs !== 'string' || Number.isNaN(Date.parse(rawTs))) {
      errors.push('timestamp must be a valid ISO date string');
    } else {
      timestamp = Date.parse(rawTs);
      const now = Date.now();
      if (now - timestamp > TIMINGS.LOCATION_MAX_AGE_MS) {
        errors.push('timestamp is too old');
      } else if (timestamp - now > 5000) {
        errors.push('timestamp is in the future');
      }
    }

    if (errors.length > 0) return { valid: false, errors };

    return {
      valid: true,
      deliveryId,
      lat,
      lng,
      accuracy: payload.accuracy,
      speed: payload.speed,
      heading: payload.heading,
      timestamp,
    };
  }
}
