import { Injectable, NotFoundException } from '@nestjs/common';
import { I18nService } from 'nestjs-i18n';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Delivery } from '../entities/delivery.entity';
import { DeliveryStatusHistory } from '../entities/delivery-status-history.entity';
import { DeliveryStatus } from '../enums/delivery-status.enum';

@Injectable()
export class DeliveryRepository {
  constructor(
    private readonly i18n: I18nService,
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
    if (!delivery)
      throw new NotFoundException(
        await this.i18n.t('delivery.notFound', { args: { id } }),
      );
    return delivery;
  }
  findByCustomerId(
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
  
  appendHistory(
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
