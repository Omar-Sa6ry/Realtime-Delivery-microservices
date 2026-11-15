import { LocationValidator } from './location.validator';
import { TIMINGS } from '../../../common/common-types/constants';

describe('LocationValidator', () => {
  let validator: LocationValidator;

  const validPayload = () => ({
    deliveryId: 'delivery_1234',
    lat: 30.0444,
    lng: 31.2357,
    timestamp: new Date().toISOString(),
  });

  beforeEach(() => {
    validator = new LocationValidator();
  });

  it('accepts a valid payload', () => {
    const result = validator.validate(validPayload());
    expect(result.valid).toBe(true);
    expect(result.errors).toBeUndefined();
    expect(result.deliveryId).toBe('delivery_1234');
    expect(result.lat).toBe(30.0444);
    expect(result.lng).toBe(31.2357);
    expect(typeof result.timestamp).toBe('number');
  });

  it('accepts optional accuracy, speed and heading', () => {
    const result = validator.validate({
      ...validPayload(),
      accuracy: 5,
      speed: 12.5,
      heading: 90,
    });
    expect(result.valid).toBe(true);
    expect(result.accuracy).toBe(5);
    expect(result.speed).toBe(12.5);
    expect(result.heading).toBe(90);
  });

  it('rejects a missing or malformed deliveryId', () => {
    expect(
      validator.validate({ ...validPayload(), deliveryId: undefined as any })
        .valid,
    ).toBe(false);
    expect(
      validator.validate({ ...validPayload(), deliveryId: 'short' }).valid,
    ).toBe(false);
    expect(
      validator.validate({ ...validPayload(), deliveryId: 'x'.repeat(65) })
        .valid,
    ).toBe(false);
    expect(
      validator.validate({ ...validPayload(), deliveryId: 'bad id!!' }).valid,
    ).toBe(false);
  });

  it('accepts deliveryId with dashes, underscores and alphanumerics', () => {
    expect(
      validator.validate({ ...validPayload(), deliveryId: 'ABC-def_12345678' })
        .valid,
    ).toBe(true);
  });

  it('rejects latitude out of range or not finite', () => {
    expect(validator.validate({ ...validPayload(), lat: 91 }).valid).toBe(
      false,
    );
    expect(validator.validate({ ...validPayload(), lat: -91 }).valid).toBe(
      false,
    );
    expect(validator.validate({ ...validPayload(), lat: NaN }).valid).toBe(
      false,
    );
    expect(
      validator.validate({ ...validPayload(), lat: 'x' as any }).valid,
    ).toBe(false);
  });

  it('rejects longitude out of range or not finite', () => {
    expect(validator.validate({ ...validPayload(), lng: 181 }).valid).toBe(
      false,
    );
    expect(validator.validate({ ...validPayload(), lng: -181 }).valid).toBe(
      false,
    );
    expect(validator.validate({ ...validPayload(), lng: Infinity }).valid).toBe(
      false,
    );
  });

  it('accepts latitude/longitude aliases', () => {
    const result = validator.validate({
      deliveryId: 'delivery_1234',
      latitude: 10,
      longitude: 20,
      timestamp: new Date().toISOString(),
    } as any);
    expect(result.valid).toBe(true);
    expect(result.lat).toBe(10);
    expect(result.lng).toBe(20);
  });

  it('rejects negative or non-finite optional fields', () => {
    expect(validator.validate({ ...validPayload(), accuracy: -1 }).valid).toBe(
      false,
    );
    expect(validator.validate({ ...validPayload(), speed: -0.1 }).valid).toBe(
      false,
    );
    expect(validator.validate({ ...validPayload(), heading: NaN }).valid).toBe(
      false,
    );
    expect(
      validator.validate({ ...validPayload(), heading: '90' as any }).valid,
    ).toBe(false);
  });

  it('rejects an invalid timestamp', () => {
    expect(
      validator.validate({ ...validPayload(), timestamp: 'not-a-date' }).valid,
    ).toBe(false);
  });

  it('rejects a timestamp that is too old', () => {
    const old = new Date(
      Date.now() - TIMINGS.LOCATION_MAX_AGE_MS - 1000,
    ).toISOString();
    const result = validator.validate({ ...validPayload(), timestamp: old });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain('timestamp is too old');
  });

  it('rejects a timestamp too far in the future', () => {
    const future = new Date(Date.now() + 10_000).toISOString();
    const result = validator.validate({ ...validPayload(), timestamp: future });
    expect(result.valid).toBe(false);
    expect(result.errors).toContain('timestamp is in the future');
  });

  it('accepts a timestamp slightly in the future (clock skew <= 5s)', () => {
    const soon = new Date(Date.now() + 3000).toISOString();
    expect(
      validator.validate({ ...validPayload(), timestamp: soon }).valid,
    ).toBe(true);
  });

  it('defaults the timestamp to now when omitted', () => {
    const payload = validPayload() as any;
    delete payload.timestamp;
    const result = validator.validate(payload);
    expect(result.valid).toBe(true);
  });

  it('collects multiple errors at once', () => {
    const result = validator.validate({
      deliveryId: 'bad',
      lat: 1000,
      lng: 1000,
      timestamp: 'nope',
    } as any);
    expect(result.valid).toBe(false);
    expect(result.errors!.length).toBeGreaterThanOrEqual(4);
  });

  it('handles null/undefined payload gracefully', () => {
    expect(validator.validate(null as any).valid).toBe(false);
    expect(validator.validate(undefined as any).valid).toBe(false);
  });
});
