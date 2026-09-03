import { Injectable, Logger } from '@nestjs/common';
import { I18nService, I18nContext } from 'nestjs-i18n';
import { WsErrorCode, WsException } from '@delivery/common';
import { SubscriptionStore } from './subscription.store';
import { RealtimeAuthorizationService } from '../../gateway/authorization/realtime-authorization.service';
import { IJwtPayload, Role } from '@delivery/common';

@Injectable()
export class SubscriptionService {
  private readonly logger = new Logger(SubscriptionService.name);

  constructor(
    private readonly store: SubscriptionStore,
    private readonly authorization: RealtimeAuthorizationService,
    private readonly i18n: I18nService,
  ) {}

  /** Authorize + subscribe a socket to a delivery channel. */
  async subscribeToDelivery(
    socketId: string,
    user: IJwtPayload,
    deliveryId: string,
  ): Promise<void> {
    const canSubscribe = await this.authorization.canSubscribeToDelivery(
      user,
      deliveryId,
    );
    if (!canSubscribe) {
      throw new WsException(
        WsErrorCode.FORBIDDEN,
        this.translate(
          user.role === Role.DRIVER
            ? 'messages.ws.notAssignedDriver'
            : 'messages.ws.notParticipant',
        ),
        false,
      );
    }
    await this.store.addSubscriber(deliveryId, socketId);
  }

  async unsubscribeFromDelivery(
    socketId: string,
    deliveryId: string,
  ): Promise<void> {
    await this.store.removeSubscriber(deliveryId, socketId);
  }

  async cleanupSocket(socketId: string): Promise<string[]> {
    return this.store.removeSocketFromAllSubscriptions(socketId);
  }

  async getDeliverySubscribers(deliveryId: string): Promise<string[]> {
    return this.store.getDeliverySubscribers(deliveryId);
  }

  private translate(key: string): string {
    const ctx = I18nContext.current();
    const lang = ctx?.lang || 'en';
    return String(this.i18n.t(key, { lang })) || key;
  }
}