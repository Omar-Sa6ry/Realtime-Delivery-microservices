import { addressFromInput, deliveryToGraphql } from './delivery.mapper';
import { Delivery } from '../entities/delivery.entity';
import { Address } from '../entities/address.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { PaymentStatus } from '../enums/payment-status.enum';
import { AddressInput } from './delivery.dto';
import { mockDelivery, createMockAddress } from '../../../test-utils';

describe('DeliveryMapper', () => {
  describe('addressFromInput', () => {
    it('should create Address entity from input', () => {
      const input: AddressInput = {
        line1: '123 Test St',
        line2: 'Apt 4B',
        city: 'San Francisco',
        state: 'CA',
        postalCode: '94105',
        countryCode: 'US',
        latitude: 37.7749,
        longitude: -122.4194,
      };

      const result = addressFromInput(input);

      expect(result).toBeInstanceOf(Address);
      expect(result.line1).toBe('123 Test St');
      expect(result.line2).toBe('Apt 4B');
      expect(result.city).toBe('San Francisco');
      expect(result.state).toBe('CA');
      expect(result.postalCode).toBe('94105');
      expect(result.countryCode).toBe('US');
      expect(result.latitude).toBe(37.7749);
      expect(result.longitude).toBe(-122.4194);
    });

    it('should use default countryCode when not provided', () => {
      const input: AddressInput = {
        line1: '123 Test St',
        city: 'San Francisco',
      };

      const result = addressFromInput(input);

      expect(result.countryCode).toBe('US');
      expect(result.line2).toBeNull();
      expect(result.state).toBeNull();
      expect(result.postalCode).toBeNull();
      expect(result.latitude).toBeNull();
      expect(result.longitude).toBeNull();
    });

    it('should handle optional fields as undefined', () => {
      const input: AddressInput = {
        line1: '123 Test St',
        city: 'San Francisco',
        line2: undefined,
        state: undefined,
        postalCode: undefined,
        countryCode: undefined,
        latitude: undefined,
        longitude: undefined,
      };

      const result = addressFromInput(input);

      expect(result.line2).toBeNull();
      expect(result.state).toBeNull();
      expect(result.postalCode).toBeNull();
      expect(result.countryCode).toBe('US');
      expect(result.latitude).toBeNull();
      expect(result.longitude).toBeNull();
    });
  });

  describe('deliveryToGraphql', () => {
    it('should map delivery entity to GraphQL type', () => {
      const delivery = mockDelivery({
        id: 'delivery-123',
        customerId: 'customer-123',
        driverId: 'driver-123',
        status: DeliveryStatus.PICKED_UP,
        paymentStatus: PaymentStatus.AUTHORIZED,
        checkoutUrl: 'https://checkout.example.com/123',
        amount: '100.00',
        currency: 'USD',
        pickupAddress: createMockAddress(),
        dropoffAddress: createMockAddress({ line1: '456 Destination St' }),
        pickedUpAt: new Date('2024-01-01T10:00:00Z'),
        completedAt: null,
        cancelledAt: null,
        createdAt: new Date('2024-01-01T09:00:00Z'),
        updatedAt: new Date('2024-01-01T10:00:00Z'),
        statusHistory: [
          { status: DeliveryStatus.CREATED, changedBy: 'system', note: 'Created', createdAt: new Date('2024-01-01T09:00:00Z') },
          { status: DeliveryStatus.PICKED_UP, changedBy: 'driver-123', note: 'Picked up', createdAt: new Date('2024-01-01T10:00:00Z') },
        ],
      });

      const result = deliveryToGraphql(delivery);

      expect(result).toEqual({
        id: 'delivery-123',
        customer: { id: 'customer-123' },
        driver: { id: 'driver-123' },
        status: DeliveryStatus.PICKED_UP,
        paymentStatus: PaymentStatus.AUTHORIZED,
        checkoutUrl: 'https://checkout.example.com/123',
        amount: '100.00',
        currency: 'USD',
        pickupAddress: {
          line1: '123 Source St',
          line2: 'Apt 1',
          city: 'San Francisco',
          state: 'CA',
          postalCode: '94105',
          countryCode: 'US',
          latitude: 37.7749,
          longitude: -122.4194,
        },
        dropoffAddress: {
          line1: '456 Destination St',
          line2: 'Apt 1',
          city: 'San Francisco',
          state: 'CA',
          postalCode: '94105',
          countryCode: 'US',
          latitude: 37.7749,
          longitude: -122.4194,
        },
        statusHistory: [
          { status: DeliveryStatus.CREATED, changedBy: 'system', note: 'Created', createdAt: new Date('2024-01-01T09:00:00Z') },
          { status: DeliveryStatus.PICKED_UP, changedBy: 'driver-123', note: 'Picked up', createdAt: new Date('2024-01-01T10:00:00Z') },
        ],
        pickedUpAt: new Date('2024-01-01T10:00:00Z'),
        completedAt: undefined,
        cancelledAt: undefined,
        createdAt: new Date('2024-01-01T09:00:00Z'),
        updatedAt: new Date('2024-01-01T10:00:00Z'),
      });
    });

    it('should handle delivery without driver', () => {
      const delivery = mockDelivery({ driverId: null });

      const result = deliveryToGraphql(delivery);

      expect(result.driver).toBeUndefined();
    });

    it('should handle delivery without checkoutUrl', () => {
      const delivery = mockDelivery({ checkoutUrl: null });

      const result = deliveryToGraphql(delivery);

      expect(result.checkoutUrl).toBeUndefined();
    });

    it('should handle delivery without statusHistory', () => {
      const delivery = mockDelivery({ statusHistory: undefined });

      const result = deliveryToGraphql(delivery);

      expect(result.statusHistory).toBeUndefined();
    });

    it('should handle delivery with empty statusHistory', () => {
      const delivery = mockDelivery({ statusHistory: [] });

      const result = deliveryToGraphql(delivery);

      expect(result.statusHistory).toEqual([]);
    });

    it('should handle date strings in statusHistory', () => {
      const delivery = mockDelivery({
        statusHistory: [
          { status: DeliveryStatus.CREATED, changedBy: 'system', note: 'Created', createdAt: '2024-01-01T09:00:00Z' },
        ],
      });

      const result = deliveryToGraphql(delivery);

      expect(result.statusHistory?.[0].createdAt).toEqual(new Date('2024-01-01T09:00:00Z'));
    });

    it('should handle undefined dates in statusHistory', () => {
      const delivery = mockDelivery({
        statusHistory: [
          { status: DeliveryStatus.CREATED, changedBy: 'system', note: 'Created', createdAt: undefined as any },
        ],
      });

      const result = deliveryToGraphql(delivery);

      expect(result.statusHistory?.[0].createdAt).toBeUndefined();
    });

    it('should handle all terminal dates', () => {
      const delivery = mockDelivery({
        pickedUpAt: new Date('2024-01-01T10:00:00Z'),
        completedAt: new Date('2024-01-01T11:00:00Z'),
        cancelledAt: new Date('2024-01-01T12:00:00Z'),
      });

      const result = deliveryToGraphql(delivery);

      expect(result.pickedUpAt).toEqual(new Date('2024-01-01T10:00:00Z'));
      expect(result.completedAt).toEqual(new Date('2024-01-01T11:00:00Z'));
      expect(result.cancelledAt).toEqual(new Date('2024-01-01T12:00:00Z'));
    });

    it('should handle date strings for terminal dates', () => {
      const delivery = mockDelivery({
        pickedUpAt: '2024-01-01T10:00:00Z',
        completedAt: '2024-01-01T11:00:00Z',
        cancelledAt: '2024-01-01T12:00:00Z',
      });

      const result = deliveryToGraphql(delivery);

      expect(result.pickedUpAt).toEqual(new Date('2024-01-01T10:00:00Z'));
      expect(result.completedAt).toEqual(new Date('2024-01-01T11:00:00Z'));
      expect(result.cancelledAt).toEqual(new Date('2024-01-01T12:00:00Z'));
    });

    it('should handle address with undefined optional fields', () => {
      const delivery = mockDelivery({
        pickupAddress: { line1: '123 St', line2: null, city: 'SF', state: null, postalCode: null, countryCode: 'US', latitude: null, longitude: null },
        dropoffAddress: { line1: '456 Ave', line2: null, city: 'LA', state: null, postalCode: null, countryCode: 'US', latitude: null, longitude: null },
      });

      const result = deliveryToGraphql(delivery);

      expect(result.pickupAddress.line2).toBeUndefined();
      expect(result.pickupAddress.state).toBeUndefined();
      expect(result.pickupAddress.postalCode).toBeUndefined();
      expect(result.pickupAddress.latitude).toBeUndefined();
      expect(result.pickupAddress.longitude).toBeUndefined();
    });
  });
});