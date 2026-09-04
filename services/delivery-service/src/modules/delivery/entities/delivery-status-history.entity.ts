import {
  Column,
  CreateDateColumn,
  Entity,
  Index,
  ManyToOne,
  PrimaryColumn,
  JoinColumn,
} from 'typeorm';
import { Delivery } from './delivery.entity';
import { IdGenerator } from '@bts-soft/common';
import { DeliveryStatus } from '../enums/delivery-status.enum';

@Entity({ name: 'delivery_status_history' })
export class DeliveryStatusHistory {
  @PrimaryColumn({ type: 'varchar', length: 20 })
  id: string = IdGenerator.generate('snowflake');

  @Index()
  @Column({ type: 'varchar', length: 20 })
  deliveryId!: string;

  @Column({ type: 'enum', enum: DeliveryStatus })
  status!: DeliveryStatus;

  @Column({ type: 'varchar', length: 120, nullable: true })
  changedBy!: string | null;

  @Column({ type: 'varchar', length: 500, nullable: true })
  note!: string | null;

  @Column({ type: 'jsonb', nullable: true })
  metadata!: Record<string, unknown> | null;

  @ManyToOne(() => Delivery, (delivery) => delivery.statusHistory, {
    onDelete: 'CASCADE',
  })
  @JoinColumn({ name: 'deliveryId' })
  delivery!: Delivery;

  @CreateDateColumn({ type: 'timestamp with time zone' })
  createdAt!: Date;
}
