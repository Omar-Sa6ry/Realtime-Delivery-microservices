import { BadRequestException } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { I18nService } from 'nestjs-i18n';
import { DeliveryStateMachine } from './delivery.state-machine';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { createMockI18nService } from '../../../test-utils';

describe('DeliveryStateMachine', () => {
  let service: DeliveryStateMachine;
  let i18nService: ReturnType<typeof createMockI18nService>;

  beforeEach(async () => {
    i18nService = createMockI18nService();
    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeliveryStateMachine,
        { provide: I18nService, useValue: i18nService },
      ],
    }).compile();

    service = module.get<DeliveryStateMachine>(DeliveryStateMachine);
  });

  it('should be defined', () => {
    expect(service).toBeDefined();
  });

  describe('canTransition', () => {
    it('should allow valid transitions from CREATED', () => {
      expect(service.canTransition(DeliveryStatus.CREATED, DeliveryStatus.PENDING_PAYMENT)).toBe(true);
      expect(service.canTransition(DeliveryStatus.CREATED, DeliveryStatus.PAYMENT_CONFIRMED)).toBe(true);
      expect(service.canTransition(DeliveryStatus.CREATED, DeliveryStatus.DRIVER_ASSIGNED)).toBe(true);
      expect(service.canTransition(DeliveryStatus.CREATED, DeliveryStatus.CANCELLED)).toBe(true);
    });

    it('should allow valid transitions from PENDING_PAYMENT', () => {
      expect(service.canTransition(DeliveryStatus.PENDING_PAYMENT, DeliveryStatus.PAYMENT_CONFIRMED)).toBe(true);
      expect(service.canTransition(DeliveryStatus.PENDING_PAYMENT, DeliveryStatus.CANCELLED)).toBe(true);
      expect(service.canTransition(DeliveryStatus.PENDING_PAYMENT, DeliveryStatus.FAILED)).toBe(true);
    });

    it('should allow valid transitions from PAYMENT_CONFIRMED', () => {
      expect(service.canTransition(DeliveryStatus.PAYMENT_CONFIRMED, DeliveryStatus.DRIVER_ASSIGNED)).toBe(true);
      expect(service.canTransition(DeliveryStatus.PAYMENT_CONFIRMED, DeliveryStatus.CANCELLED)).toBe(true);
    });

    it('should allow valid transitions from DRIVER_ASSIGNED', () => {
      expect(service.canTransition(DeliveryStatus.DRIVER_ASSIGNED, DeliveryStatus.DRIVER_ACCEPTED)).toBe(true);
      expect(service.canTransition(DeliveryStatus.DRIVER_ASSIGNED, DeliveryStatus.CANCELLED)).toBe(true);
    });

    it('should allow valid transitions from DRIVER_ACCEPTED', () => {
      expect(service.canTransition(DeliveryStatus.DRIVER_ACCEPTED, DeliveryStatus.PICKED_UP)).toBe(true);
      expect(service.canTransition(DeliveryStatus.DRIVER_ACCEPTED, DeliveryStatus.CANCELLED)).toBe(true);
    });

    it('should allow valid transitions from PICKED_UP', () => {
      expect(service.canTransition(DeliveryStatus.PICKED_UP, DeliveryStatus.IN_TRANSIT)).toBe(true);
      expect(service.canTransition(DeliveryStatus.PICKED_UP, DeliveryStatus.CANCELLED)).toBe(true);
    });

    it('should allow valid transitions from IN_TRANSIT', () => {
      expect(service.canTransition(DeliveryStatus.IN_TRANSIT, DeliveryStatus.DELIVERED)).toBe(true);
      expect(service.canTransition(DeliveryStatus.IN_TRANSIT, DeliveryStatus.FAILED)).toBe(true);
    });

    it('should allow valid transitions from DELIVERED', () => {
      expect(service.canTransition(DeliveryStatus.DELIVERED, DeliveryStatus.COMPLETED)).toBe(true);
    });

    it('should not allow transitions from terminal states', () => {
      expect(service.canTransition(DeliveryStatus.COMPLETED, DeliveryStatus.CANCELLED)).toBe(false);
      expect(service.canTransition(DeliveryStatus.CANCELLED, DeliveryStatus.COMPLETED)).toBe(false);
      expect(service.canTransition(DeliveryStatus.FAILED, DeliveryStatus.COMPLETED)).toBe(false);
    });

    it('should not allow invalid transitions', () => {
      expect(service.canTransition(DeliveryStatus.CREATED, DeliveryStatus.PICKED_UP)).toBe(false);
      expect(service.canTransition(DeliveryStatus.PENDING_PAYMENT, DeliveryStatus.DRIVER_ASSIGNED)).toBe(false);
      expect(service.canTransition(DeliveryStatus.PAYMENT_CONFIRMED, DeliveryStatus.PICKED_UP)).toBe(false);
      expect(service.canTransition(DeliveryStatus.DRIVER_ASSIGNED, DeliveryStatus.PICKED_UP)).toBe(false);
      expect(service.canTransition(DeliveryStatus.DRIVER_ACCEPTED, DeliveryStatus.IN_TRANSIT)).toBe(false);
      expect(service.canTransition(DeliveryStatus.PICKED_UP, DeliveryStatus.DELIVERED)).toBe(false);
      expect(service.canTransition(DeliveryStatus.IN_TRANSIT, DeliveryStatus.COMPLETED)).toBe(false);
    });

    it('should return false for same state transition', () => {
      expect(service.canTransition(DeliveryStatus.CREATED, DeliveryStatus.CREATED)).toBe(false);
      expect(service.canTransition(DeliveryStatus.PAYMENT_CONFIRMED, DeliveryStatus.PAYMENT_CONFIRMED)).toBe(false);
    });
  });

  describe('assertTransition', () => {
    it('should not throw for same state', async () => {
      await expect(service.assertTransition(DeliveryStatus.CREATED, DeliveryStatus.CREATED)).resolves.not.toThrow();
    });

    it('should not throw for valid transition', async () => {
      await expect(service.assertTransition(DeliveryStatus.CREATED, DeliveryStatus.PAYMENT_CONFIRMED)).resolves.not.toThrow();
    });

    it('should throw BadRequestException for invalid transition', async () => {
      i18nService.t.mockResolvedValue('Invalid transition from CREATED to PICKED_UP');
      
      await expect(service.assertTransition(DeliveryStatus.CREATED, DeliveryStatus.PICKED_UP))
        .rejects.toThrow(BadRequestException);
      
      expect(i18nService.t).toHaveBeenCalledWith('delivery.invalidTransition', {
        args: { from: DeliveryStatus.CREATED, to: DeliveryStatus.PICKED_UP },
      });
    });

    it('should throw BadRequestException with translated message', async () => {
      i18nService.t.mockResolvedValue('انتقال غير صالح من CREATED إلى PICKED_UP');
      
      try {
        await service.assertTransition(DeliveryStatus.CREATED, DeliveryStatus.PICKED_UP);
      } catch (error) {
        expect(error).toBeInstanceOf(BadRequestException);
        expect(error.message).toBe('انتقال غير صالح من CREATED إلى PICKED_UP');
      }
    });
  });

  describe('nextStates', () => {
    it('should return valid next states for each state', () => {
      expect(service.nextStates(DeliveryStatus.CREATED)).toEqual([
        DeliveryStatus.PENDING_PAYMENT,
        DeliveryStatus.PAYMENT_CONFIRMED,
        DeliveryStatus.DRIVER_ASSIGNED,
        DeliveryStatus.CANCELLED,
      ]);

      expect(service.nextStates(DeliveryStatus.PENDING_PAYMENT)).toEqual([
        DeliveryStatus.PAYMENT_CONFIRMED,
        DeliveryStatus.CANCELLED,
        DeliveryStatus.FAILED,
      ]);

      expect(service.nextStates(DeliveryStatus.PAYMENT_CONFIRMED)).toEqual([
        DeliveryStatus.DRIVER_ASSIGNED,
        DeliveryStatus.CANCELLED,
      ]);

      expect(service.nextStates(DeliveryStatus.DRIVER_ASSIGNED)).toEqual([
        DeliveryStatus.DRIVER_ACCEPTED,
        DeliveryStatus.CANCELLED,
      ]);

      expect(service.nextStates(DeliveryStatus.DRIVER_ACCEPTED)).toEqual([
        DeliveryStatus.PICKED_UP,
        DeliveryStatus.CANCELLED,
      ]);

      expect(service.nextStates(DeliveryStatus.PICKED_UP)).toEqual([
        DeliveryStatus.IN_TRANSIT,
        DeliveryStatus.CANCELLED,
      ]);

      expect(service.nextStates(DeliveryStatus.IN_TRANSIT)).toEqual([
        DeliveryStatus.DELIVERED,
        DeliveryStatus.FAILED,
      ]);

      expect(service.nextStates(DeliveryStatus.DELIVERED)).toEqual([
        DeliveryStatus.COMPLETED,
      ]);

      expect(service.nextStates(DeliveryStatus.COMPLETED)).toEqual([]);
      expect(service.nextStates(DeliveryStatus.CANCELLED)).toEqual([]);
      expect(service.nextStates(DeliveryStatus.FAILED)).toEqual([]);
    });

    it('should return empty array for unknown state', () => {
      // @ts-expect-error - testing unknown state
      expect(service.nextStates('UNKNOWN_STATE')).toEqual([]);
    });
  });
});