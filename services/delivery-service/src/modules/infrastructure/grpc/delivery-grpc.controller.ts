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
}
