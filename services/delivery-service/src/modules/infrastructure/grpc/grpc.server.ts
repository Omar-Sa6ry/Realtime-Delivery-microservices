import { Controller } from '@nestjs/common';
import { GrpcMethod } from '@nestjs/microservices';
import { DeliveryRepository } from '../../delivery/repositories/delivery.repository';

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

interface RefundPaymentRequest {
  deliveryId: string;
  reason: string;
}
interface RefundPaymentResponse {
  accepted: boolean;
  deliveryId: string;
  refundId: string;
}

@Controller()
export class GrpcServer {
  constructor(private readonly repository: DeliveryRepository) {}

  @GrpcMethod('DeliveryService', 'IsParticipant')
  async isParticipant(
    request: ParticipantRequest,
  ): Promise<ParticipantResponse> {
    try {
      const delivery = await this.repository.findById(request.deliveryId);
      return {
        isParticipant:
          delivery.customerId === request.userId ||
          delivery.driverId === request.userId,
      };
    } catch {
      return { isParticipant: false };
    }
  }

  @GrpcMethod('DeliveryService', 'GetDelivery')
  async getDelivery(
    request: GetDeliveryRequest,
  ): Promise<GetDeliveryResponse> {
    try {
      const delivery = await this.repository.findById(request.deliveryId);
      return {
        found: true,
        deliveryId: delivery.id,
        customerId: delivery.customerId,
        driverId: delivery.driverId || '',
        status: delivery.status,
        amount: Number(delivery.amount) || 0,
        currency: delivery.currency || 'USD',
      };
    } catch {
      return {
        found: false,
        deliveryId: request.deliveryId,
        customerId: '',
        driverId: '',
        status: '',
        amount: 0,
        currency: '',
      };
    }
  }

  @GrpcMethod('DeliveryService', 'RefundPayment')
  async refundPayment(
    request: RefundPaymentRequest,
  ): Promise<RefundPaymentResponse> {
    return {
      accepted: true,
      deliveryId: request.deliveryId,
      refundId: `ref_${Date.now()}`,
    };
  }
}

