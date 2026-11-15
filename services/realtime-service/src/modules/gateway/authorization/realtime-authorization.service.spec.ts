import { RealtimeAuthorizationService } from './realtime-authorization.service';
import { DeliveryPolicy } from './policies/delivery.policy';
import { DriverPolicy } from './policies/driver.policy';
import { IJwtPayload, Role, WsErrorCode } from '@delivery-micro/shard';

describe('RealtimeAuthorizationService', () => {
  let service: RealtimeAuthorizationService;
  let deliveryPolicy: { isParticipant: jest.Mock };
  let driverPolicy: { isAssignedDriver: jest.Mock };

  const admin: IJwtPayload = { userId: 'adm1', role: Role.ADMIN };
  const driver: IJwtPayload = { userId: 'drv1', role: Role.DRIVER };
  const customer: IJwtPayload = { userId: 'cust1', role: Role.USER };

  beforeEach(() => {
    deliveryPolicy = { isParticipant: jest.fn().mockResolvedValue(true) };
    driverPolicy = { isAssignedDriver: jest.fn().mockResolvedValue(true) };
    service = new RealtimeAuthorizationService(
      deliveryPolicy as unknown as DeliveryPolicy,
      driverPolicy as unknown as DriverPolicy,
    );
  });

  describe('canSubscribeToDelivery', () => {
    it('lets admins subscribe to any delivery', async () => {
      await expect(service.canSubscribeToDelivery(admin, 'd1')).resolves.toBe(
        true,
      );
      expect(deliveryPolicy.isParticipant).not.toHaveBeenCalled();
      expect(driverPolicy.isAssignedDriver).not.toHaveBeenCalled();
    });

    it('checks driver assignment for drivers', async () => {
      await expect(service.canSubscribeToDelivery(driver, 'd1')).resolves.toBe(
        true,
      );
      expect(driverPolicy.isAssignedDriver).toHaveBeenCalledWith('drv1', 'd1');
    });

    it('checks participation for customers', async () => {
      await expect(
        service.canSubscribeToDelivery(customer, 'd1'),
      ).resolves.toBe(true);
      expect(deliveryPolicy.isParticipant).toHaveBeenCalledWith('cust1', 'd1');
    });

    it('denies when the user id cannot be resolved', async () => {
      await expect(
        service.canSubscribeToDelivery({ role: Role.USER }, 'd1'),
      ).resolves.toBe(false);
    });

    it('supports sub and id fallbacks for user id', async () => {
      await service.canSubscribeToDelivery(
        { sub: 's1', role: Role.USER },
        'd1',
      );
      expect(deliveryPolicy.isParticipant).toHaveBeenCalledWith('s1', 'd1');

      await service.canSubscribeToDelivery(
        { id: 'i1', role: Role.USER } as IJwtPayload,
        'd1',
      );
      expect(deliveryPolicy.isParticipant).toHaveBeenCalledWith('i1', 'd1');
    });

    it('propagates policy denials', async () => {
      driverPolicy.isAssignedDriver.mockResolvedValue(false);
      await expect(service.canSubscribeToDelivery(driver, 'd1')).resolves.toBe(
        false,
      );
    });
  });

  describe('canSendLocationUpdate', () => {
    it('delegates to the driver policy', async () => {
      await expect(service.canSendLocationUpdate('drv1', 'd1')).resolves.toBe(
        true,
      );
      expect(driverPolicy.isAssignedDriver).toHaveBeenCalledWith('drv1', 'd1');
    });
  });

  describe('canSendCommand', () => {
    it('lets admins send any command', async () => {
      await expect(
        service.canSendCommand(admin, 'ACCEPT_ASSIGNMENT', 'd1'),
      ).resolves.toBe(true);
    });

    it('denies non-driver users', async () => {
      await expect(
        service.canSendCommand(customer, 'ACCEPT_ASSIGNMENT', 'd1'),
      ).resolves.toBe(false);
      expect(driverPolicy.isAssignedDriver).not.toHaveBeenCalled();
    });

    it('checks assignment for drivers', async () => {
      await expect(
        service.canSendCommand(driver, 'COMPLETE_DELIVERY', 'd1'),
      ).resolves.toBe(true);
      expect(driverPolicy.isAssignedDriver).toHaveBeenCalledWith('drv1', 'd1');
    });

    it('denies when the driver is not assigned', async () => {
      driverPolicy.isAssignedDriver.mockResolvedValue(false);
      await expect(
        service.canSendCommand(driver, 'REJECT_ASSIGNMENT', 'd1'),
      ).resolves.toBe(false);
    });
  });

  describe('assertCanSubscribe', () => {
    it('resolves when authorized', async () => {
      await expect(
        service.assertCanSubscribe(admin, 'd1'),
      ).resolves.toBeUndefined();
    });

    it('throws FORBIDDEN when not authorized', async () => {
      deliveryPolicy.isParticipant.mockResolvedValue(false);
      await expect(
        service.assertCanSubscribe(customer, 'd1'),
      ).rejects.toMatchObject({ code: WsErrorCode.FORBIDDEN });
    });
  });
});
