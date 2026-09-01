import { Delivery } from '../../delivery/entities/delivery.entity';
import { Address } from '../../delivery/entities/address.entity';
import { AddressInput } from './delivery.dto';
import { DeliveryType } from './delivery.types';

export function addressFromInput(input: AddressInput): Address {
  const address = new Address();
  address.line1 = input.line1;
  address.line2 = input.line2 ?? null;
  address.city = input.city;
  address.state = input.state ?? null;
  address.postalCode = input.postalCode ?? null;
  address.countryCode = input.countryCode ?? 'US';
  address.latitude = input.latitude ?? null;
  address.longitude = input.longitude ?? null;
  return address;
}

export function deliveryToGraphql(delivery: Delivery): DeliveryType {
  return {
    id: delivery.id,
    customerId: delivery.customerId,
    driverId: delivery.driverId ?? undefined,
    status: delivery.status,
    paymentStatus: delivery.paymentStatus,
    amount: delivery.amount,
    currency: delivery.currency,
    pickupAddress: {
      line1: delivery.pickupAddress.line1,
      line2: delivery.pickupAddress.line2 ?? undefined,
      city: delivery.pickupAddress.city,
      state: delivery.pickupAddress.state ?? undefined,
      postalCode: delivery.pickupAddress.postalCode ?? undefined,
      countryCode: delivery.pickupAddress.countryCode,
      latitude: delivery.pickupAddress.latitude ?? undefined,
      longitude: delivery.pickupAddress.longitude ?? undefined,
    },
    dropoffAddress: {
      line1: delivery.dropoffAddress.line1,
      line2: delivery.dropoffAddress.line2 ?? undefined,
      city: delivery.dropoffAddress.city,
      state: delivery.dropoffAddress.state ?? undefined,
      postalCode: delivery.dropoffAddress.postalCode ?? undefined,
      countryCode: delivery.dropoffAddress.countryCode,
      latitude: delivery.dropoffAddress.latitude ?? undefined,
      longitude: delivery.dropoffAddress.longitude ?? undefined,
    },
    statusHistory: delivery.statusHistory?.map((h) => ({
      status: h.status,
      changedBy: h.changedBy ?? undefined,
      note: h.note ?? undefined,
      createdAt: h.createdAt,
    })) ?? undefined,
    pickedUpAt: delivery.pickedUpAt ?? undefined,
    completedAt: delivery.completedAt ?? undefined,
    cancelledAt: delivery.cancelledAt ?? undefined,
    createdAt: delivery.createdAt,
    updatedAt: delivery.updatedAt,
  };
}
