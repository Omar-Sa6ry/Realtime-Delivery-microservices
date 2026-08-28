import { Controller } from '@nestjs/common';
import { GrpcMethod } from '@nestjs/microservices';
import { DeliveryRepository } from '../../delivery/repositories/delivery.repository';

interface ParticipantRequest {
  userId: string;
  deliveryId: string;
}
interface ParticipantResponse {
  participant: boolean;
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
        participant:
          delivery.customerId === request.userId ||
          delivery.driverId === request.userId,
      };
    } catch {
      return { participant: false };
    }
  }
}
