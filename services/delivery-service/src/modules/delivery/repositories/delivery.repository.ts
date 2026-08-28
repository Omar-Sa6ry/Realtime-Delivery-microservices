import { Injectable, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryStatusHistory } from '../entities/delivery-status-history.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';

@Injectable()
export class DeliveryRepository {
  constructor(
    @InjectRepository(Delivery)
    private readonly deliveries: Repository<Delivery>,
    @InjectRepository(DeliveryStatusHistory)
    private readonly history: Repository<DeliveryStatusHistory>,
  ) {}

  async create(delivery: Partial<Delivery>): Promise<Delivery> {
    return this.deliveries.save(this.deliveries.create(delivery));
  }

  async save(delivery: Delivery): Promise<Delivery> {
    return this.deliveries.save(delivery);
  }

  async findById(id: string): Promise<Delivery> {
    const delivery = await this.deliveries.findOne({
      where: { id },
      relations: { statusHistory: true, sagaStates: true },
    });
    if (!delivery) throw new NotFoundException(`Delivery ${id} was not found`);
    return delivery;
  }
  
  async findByCustomerId(
    customerId: string,
    skip = 0,
    take = 50,
  ): Promise<[Delivery[], number]> {
    return this.deliveries.findAndCount({
      where: { customerId },
      order: { createdAt: 'DESC' },
      skip,
      take,
    });
  }
  async appendHistory(
    delivery: Delivery,
    status: DeliveryStatus,
    changedBy?: string,
    note?: string,
  ): Promise<DeliveryStatusHistory> {
    return this.history.save(
      this.history.create({
        deliveryId: delivery.id,
        delivery,
        status,
        changedBy: changedBy ?? null,
        note: note ?? null,
        metadata: null,
      }),
    );
  }
}
