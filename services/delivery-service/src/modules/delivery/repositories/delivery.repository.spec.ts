import { NotFoundException } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { I18nService } from 'nestjs-i18n';
import { IsNull, Repository } from 'typeorm';
import { DeliveryRepository } from './delivery.repository';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryStatusHistory } from '../entities/delivery-status-history.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { createMockI18nService, mockDelivery, createMockAddress, mockDeliveryStatusHistory } from '../../../test-utils';

describe('DeliveryRepository', () => {
  let repository: DeliveryRepository;
  let deliveriesRepo: Partial<Record<keyof Repository<Delivery>, jest.Mock>>;
  let historyRepo: Partial<Record<keyof Repository<DeliveryStatusHistory>, jest.Mock>>;
  let i18nService: ReturnType<typeof createMockI18nService>;

  beforeEach(async () => {
    deliveriesRepo = {
      create: jest.fn(),
      save: jest.fn(),
      findOne: jest.fn(),
      findAndCount: jest.fn(),
      update: jest.fn(),
      find: jest.fn(),
    };

    historyRepo = {
      create: jest.fn(),
      save: jest.fn(),
    };

    i18nService = createMockI18nService();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        DeliveryRepository,
        { provide: I18nService, useValue: i18nService },
        { provide: 'DeliveryRepository', useValue: deliveriesRepo },
        { provide: 'DeliveryStatusHistoryRepository', useValue: historyRepo },
      ],
    }).compile();

    repository = module.get<DeliveryRepository>(DeliveryRepository);
  });

  it('should be defined', () => {
    expect(repository).toBeDefined();
  });

  describe('create', () => {
    it('should create and save a new delivery', async () => {
      const deliveryData = {
        customerId: 'customer-123',
        amount: '100.00',
        currency: 'USD',
        pickupAddress: createMockAddress(),
        dropoffAddress: createMockAddress({ line1: '456 Destination St' }),
        status: DeliveryStatus.CREATED,
        paymentStatus: 'PENDING' as any,
      };

      const createdDelivery = mockDelivery({ ...deliveryData });
      deliveriesRepo.create.mockReturnValue(createdDelivery);
      deliveriesRepo.save.mockResolvedValue(createdDelivery);

      const result = await repository.create(deliveryData);

      expect(deliveriesRepo.create).toHaveBeenCalledWith(deliveryData);
      expect(deliveriesRepo.save).toHaveBeenCalledWith(createdDelivery);
      expect(result).toEqual(createdDelivery);
    });
  });

  describe('save', () => {
    it('should save the delivery entity', async () => {
      const delivery = mockDelivery();
      deliveriesRepo.save.mockResolvedValue(delivery);

      const result = await repository.save(delivery);

      expect(deliveriesRepo.save).toHaveBeenCalledWith(delivery);
      expect(result).toEqual(delivery);
    });
  });

  describe('findById', () => {
    it('should return delivery when found', async () => {
      const delivery = mockDelivery({ statusHistory: [mockDeliveryStatusHistory()] });
      deliveriesRepo.findOne.mockResolvedValue(delivery);

      const result = await repository.findById('test-delivery-id');

      expect(deliveriesRepo.findOne).toHaveBeenCalledWith({
        where: { id: 'test-delivery-id' },
        relations: { statusHistory: true, sagaState: true },
      });
      expect(result).toEqual(delivery);
    });

    it('should throw NotFoundException when delivery not found', async () => {
      deliveriesRepo.findOne.mockResolvedValue(null);
      i18nService.t.mockResolvedValue('Delivery with ID test-delivery-id not found');

      await expect(repository.findById('test-delivery-id')).rejects.toThrow(NotFoundException);
      expect(i18nService.t).toHaveBeenCalledWith('delivery.notFound', { args: { id: 'test-delivery-id' } });
    });
  });

  describe('findByCustomerId', () => {
    it('should return paginated deliveries for customer', async () => {
      const deliveries = [mockDelivery({ id: '1' }), mockDelivery({ id: '2' })];
      deliveriesRepo.findAndCount.mockResolvedValue([deliveries, 2]);

      const result = await repository.findByCustomerId('customer-123', 0, 50);

      expect(deliveriesRepo.findAndCount).toHaveBeenCalledWith({
        where: { customerId: 'customer-123' },
        order: { createdAt: 'DESC' },
        skip: 0,
        take: 50,
      });
      expect(result).toEqual([deliveries, 2]);
    });

    it('should use default pagination when not provided', async () => {
      deliveriesRepo.findAndCount.mockResolvedValue([[], 0]);

      await repository.findByCustomerId('customer-123');

      expect(deliveriesRepo.findAndCount).toHaveBeenCalledWith({
        where: { customerId: 'customer-123' },
        order: { createdAt: 'DESC' },
        skip: 0,
        take: 50,
      });
    });
  });

  describe('findUnassignedPendingDeliveries', () => {
    it('should return unassigned pending deliveries ordered by createdAt', async () => {
      const deliveries = [mockDelivery({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED, driverId: null })];
      deliveriesRepo.find.mockResolvedValue(deliveries);

      const result = await repository.findUnassignedPendingDeliveries();

      expect(deliveriesRepo.find).toHaveBeenCalledWith({
        where: {
          status: DeliveryStatus.PAYMENT_CONFIRMED,
          driverId: IsNull(),
        },
        order: { createdAt: 'ASC' },
        take: 100,
      });
      expect(result).toEqual(deliveries);
    });
  });

  describe('clearDriverId', () => {
    it('should clear the driverId for a delivery', async () => {
      deliveriesRepo.update.mockResolvedValue({ affected: 1 });

      await repository.clearDriverId('test-delivery-id');

      expect(deliveriesRepo.update).toHaveBeenCalledWith('test-delivery-id', { driverId: null });
    });
  });

  describe('findOpenDeliveries', () => {
    it('should return open deliveries with correct filters', async () => {
      const deliveries = [
        mockDelivery({ id: '1', status: DeliveryStatus.PAYMENT_CONFIRMED, driverId: null }),
        mockDelivery({ id: '2', status: DeliveryStatus.DRIVER_ASSIGNED, driverId: null }),
      ];
      deliveriesRepo.findAndCount.mockResolvedValue([deliveries, 2]);

      const result = await repository.findOpenDeliveries(0, 50);

      expect(deliveriesRepo.findAndCount).toHaveBeenCalledWith({
        where: [
          { status: DeliveryStatus.PAYMENT_CONFIRMED, driverId: IsNull() },
          { status: DeliveryStatus.DRIVER_ASSIGNED, driverId: IsNull() },
        ],
        order: { createdAt: 'DESC' },
        skip: 0,
        take: 50,
      });
      expect(result).toEqual([deliveries, 2]);
    });
  });

  describe('appendHistory', () => {
    it('should create and save history entry', async () => {
      const delivery = mockDelivery({ statusHistory: [] });
      const historyItem = mockDeliveryStatusHistory();
      historyRepo.create.mockReturnValue(historyItem);
      historyRepo.save.mockResolvedValue(historyItem);

      const result = await repository.appendHistory(delivery, DeliveryStatus.DRIVER_ASSIGNED, 'driver-123', 'Assigned');

      expect(historyRepo.create).toHaveBeenCalledWith({
        deliveryId: delivery.id,
        status: DeliveryStatus.DRIVER_ASSIGNED,
        changedBy: 'driver-123',
        note: 'Assigned',
        metadata: null,
      });
      expect(historyRepo.save).toHaveBeenCalledWith(historyItem);
      expect(delivery.statusHistory).toContain(historyItem);
      expect(result).toEqual(historyItem);
    });

    it('should use null for optional parameters when not provided', async () => {
      const delivery = mockDelivery({ statusHistory: [] });
      const historyItem = mockDeliveryStatusHistory({ changedBy: null, note: null });
      historyRepo.create.mockReturnValue(historyItem);
      historyRepo.save.mockResolvedValue(historyItem);

      await repository.appendHistory(delivery, DeliveryStatus.CANCELLED);

      expect(historyRepo.create).toHaveBeenCalledWith({
        deliveryId: delivery.id,
        status: DeliveryStatus.CANCELLED,
        changedBy: null,
        note: null,
        metadata: null,
      });
    });
  });
});