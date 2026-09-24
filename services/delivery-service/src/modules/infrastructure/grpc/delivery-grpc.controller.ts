import { Controller } from '@nestjs/common';
import { GrpcMethod } from '@nestjs/microservices';
import { DeliveryQueryService } from '../../delivery/services/delivery-query.service';

interface ParticipantRequest {
  userId: string;
  deliveryId: string;
}

interface ParticipantResponse {
  isParticipant: boolean;
}

interface GetDeliveryRequest {
  deliveryId: string;
}

interface GetDeliveryResponse {
  found: boolean;
  deliveryId: string;
  customerId: string;
  driverId: string;
  status: string;
  amount: number;
  currency: string;
}

@Controller()
export class DeliveryGrpcController {
  constructor(private readonly deliveryQueryService: DeliveryQueryService) {}

  @GrpcMethod('DeliveryService', 'IsParticipant')
  async isParticipant(data: ParticipantRequest): Promise<ParticipantResponse> {
    try {
      const delivery = await this.deliveryQueryService.getById(data.deliveryId);
      if (!delivery) {
        return { isParticipant: false };
      }
      const isParticipant =
        delivery.customerId === data.userId || delivery.driverId === data.userId;
      return { isParticipant };
    } catch {
      return { isParticipant: false };
    }
  }

  @GrpcMethod('DeliveryService', 'GetDelivery')
  async getDelivery(data: GetDeliveryRequest): Promise<GetDeliveryResponse> {
    try {
      const delivery = await this.deliveryQueryService.getById(data.deliveryId);
      if (!delivery) {
        return {
          found: false,
          deliveryId: data.deliveryId,
          customerId: '',
          driverId: '',
          status: '',
          amount: 0,
          currency: '',
        };
      }
      return {
        found: true,
        deliveryId: delivery.id,
        customerId: delivery.customerId || '',
        driverId: delivery.driverId || '',
        status: delivery.status || '',
        amount: Number(delivery.amount || 0),
        currency: delivery.currency || 'USD',
      };
    } catch {
      return {
        found: false,
        deliveryId: data.deliveryId,
        customerId: '',
        driverId: '',
        status: '',
        amount: 0,
        currency: '',
      };
    }
  }

  @GrpcMethod('DeliveryService', 'GetOpenDeliveries')
  async getOpenDeliveries(data: { page?: number; limit?: number }): Promise<{
    items: Array<{
      id: string;
      customer_id: string;
      status: string;
      pickup_city: string;
      pickup_country: string;
      dropoff_city: string;
      dropoff_country: string;
      amount: string;
      currency: string;
      created_at: string;
    }>;
    total_items: number;
  }> {
    try {
      const page = Math.max(1, data?.page || 1);
      const limit = Math.max(1, Math.min(100, data?.limit || 50));
      const [deliveries, total] = await this.deliveryQueryService.getOpenDeliveries(page, limit);

      const items = (deliveries || []).map((d) => ({
        id: d.id,
        customer_id: d.customerId || '',
        status: d.status || '',
        pickup_city: d.pickupAddress?.city || '',
        pickup_country: d.pickupAddress?.countryCode || '',
        dropoff_city: d.dropoffAddress?.city || '',
        dropoff_country: d.dropoffAddress?.countryCode || '',
        amount: d.amount ? String(d.amount) : '0',
        currency: d.currency || 'USD',
        created_at: d.createdAt ? new Date(d.createdAt).toISOString() : new Date().toISOString(),
      }));

      return {
        items,
        total_items: total,
      };
    } catch {
      return {
        items: [],
        total_items: 0,
      };
    }
  }
}

