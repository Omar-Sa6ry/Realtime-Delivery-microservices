import { Injectable, Logger } from '@nestjs/common';
import { IJwtPayload, Role } from '@delivery/common';
import { WsErrorCode, WsException } from '@delivery/common';
import { DeliveryPolicy } from './policies/delivery.policy';
import { DriverPolicy } from './policies/driver.policy';

export type WsCommandType =
  'ACCEPT_ASSIGNMENT' | 'REJECT_ASSIGNMENT' | 'COMPLETE_DELIVERY';

@Injectable()
export class RealtimeAuthorizationService {
  private readonly logger = new Logger(RealtimeAuthorizationService.name);

  constructor(
    private readonly deliveryPolicy: DeliveryPolicy,
    private readonly driverPolicy: DriverPolicy,
  ) {}

  async canSubscribeToDelivery(
    user: IJwtPayload,
    deliveryId: string,
  ): Promise<boolean> {
    if (user.role === Role.ADMIN) return true;
    const userId = this.uid(user);
    if (!userId) return false;
    if (user.role === Role.DRIVER) {
      return this.driverPolicy.isAssignedDriver(userId, deliveryId);
    }
    return this.deliveryPolicy.isParticipant(userId, deliveryId);
  }

  async canSendLocationUpdate(
    driverId: string,
    deliveryId: string,
  ): Promise<boolean> {
    return this.driverPolicy.isAssignedDriver(driverId, deliveryId);
  }

  async canSendCommand(
    user: IJwtPayload,
    command: WsCommandType,
    deliveryId: string,
  ): Promise<boolean> {
    if (user.role === Role.ADMIN) return true;
    if (user.role !== Role.DRIVER) {
      const userId = this.uid(user);
      this.logger.warn(
        `Non-driver attempted command ${command} userId=${userId} delivery=${deliveryId}`,
      );
      return false;
    }
    return this.driverPolicy.isAssignedDriver(this.uid(user), deliveryId);
  }

  private uid(user: IJwtPayload): string {
    return user.userId || user.sub || user.id || '';
  }

  async assertCanSubscribe(
    user: IJwtPayload,
    deliveryId: string,
  ): Promise<void> {
    if (!(await this.canSubscribeToDelivery(user, deliveryId))) {
      throw new WsException(WsErrorCode.FORBIDDEN);
    }
  }
}
