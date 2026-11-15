import { SubscriptionService } from './subscription.service';
import { SubscriptionStore } from './subscription.store';
import { RealtimeAuthorizationService } from '../../gateway/authorization/realtime-authorization.service';
import { I18nService } from 'nestjs-i18n';
import { IJwtPayload, Role, WsErrorCode } from '@delivery-micro/shard';

describe('SubscriptionService', () => {
  let service: SubscriptionService;
  let store: {
    addSubscriber: jest.Mock;
    removeSubscriber: jest.Mock;
    removeSocketFromAllSubscriptions: jest.Mock;
    getDeliverySubscribers: jest.Mock;
  };
  let authorization: { canSubscribeToDelivery: jest.Mock };
  let i18n: { t: jest.Mock };

  const driver: IJwtPayload = { userId: 'drv1', role: Role.DRIVER };
  const customer: IJwtPayload = { userId: 'cust1', role: Role.USER };
  const admin: IJwtPayload = { userId: 'adm1', role: Role.ADMIN };

  beforeEach(() => {
    store = {
      addSubscriber: jest.fn().mockResolvedValue(undefined),
      removeSubscriber: jest.fn().mockResolvedValue(undefined),
      removeSocketFromAllSubscriptions: jest.fn().mockResolvedValue(['d1']),
      getDeliverySubscribers: jest.fn().mockResolvedValue(['s1', 's2']),
    };
    authorization = {
      canSubscribeToDelivery: jest.fn().mockResolvedValue(true),
    };
    i18n = { t: jest.fn((key: string) => `t:${key}`) };

    service = new SubscriptionService(
      store as unknown as SubscriptionStore,
      authorization as unknown as RealtimeAuthorizationService,
      i18n as unknown as I18nService,
    );
  });

  describe('subscribeToDelivery', () => {
    it('subscribes when authorized', async () => {
      await service.subscribeToDelivery('s1', customer, 'd1');
      expect(authorization.canSubscribeToDelivery).toHaveBeenCalledWith(
        customer,
        'd1',
      );
      expect(store.addSubscriber).toHaveBeenCalledWith('d1', 's1');
    });

    it('throws forbidden for a non-participant customer with translated message', async () => {
      authorization.canSubscribeToDelivery.mockResolvedValue(false);

      await expect(
        service.subscribeToDelivery('s1', customer, 'd1'),
      ).rejects.toMatchObject({ code: WsErrorCode.FORBIDDEN });
      expect(i18n.t).toHaveBeenCalledWith('messages.ws.notParticipant', {
        lang: 'en',
      });
      expect(store.addSubscriber).not.toHaveBeenCalled();
    });

    it('uses the driver-specific message for drivers', async () => {
      authorization.canSubscribeToDelivery.mockResolvedValue(false);

      await expect(
        service.subscribeToDelivery('s1', driver, 'd1'),
      ).rejects.toMatchObject({ code: WsErrorCode.FORBIDDEN });
      expect(i18n.t).toHaveBeenCalledWith('messages.ws.notAssignedDriver', {
        lang: 'en',
      });
    });

    it('falls back to the message key when the translation is empty', async () => {
      authorization.canSubscribeToDelivery.mockResolvedValue(false);
      i18n.t.mockReturnValue('');

      await expect(
        service.subscribeToDelivery('s1', customer, 'd1'),
      ).rejects.toMatchObject({
        code: WsErrorCode.FORBIDDEN,
        message: 'messages.ws.notParticipant',
      });
    });

    it('allows admins through authorization', async () => {
      await service.subscribeToDelivery('s1', admin, 'd1');
      expect(store.addSubscriber).toHaveBeenCalledWith('d1', 's1');
    });
  });

  it('unsubscribeFromDelivery delegates to the store', async () => {
    await service.unsubscribeFromDelivery('s1', 'd1');
    expect(store.removeSubscriber).toHaveBeenCalledWith('d1', 's1');
  });

  it('cleanupSocket removes all subscriptions for a socket', async () => {
    await expect(service.cleanupSocket('s1')).resolves.toEqual(['d1']);
    expect(store.removeSocketFromAllSubscriptions).toHaveBeenCalledWith('s1');
  });

  it('getDeliverySubscribers returns the subscriber list', async () => {
    await expect(service.getDeliverySubscribers('d1')).resolves.toEqual([
      's1',
      's2',
    ]);
  });
});
