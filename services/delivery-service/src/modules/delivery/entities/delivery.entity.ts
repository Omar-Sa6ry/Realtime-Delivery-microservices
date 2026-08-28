import {
  Column,
  CreateDateColumn,
  Entity,
  Index,
  OneToMany,
  OneToOne,
  PrimaryColumn,
  UpdateDateColumn,
} from 'typeorm';
import { Address } from './address.entity';
import { IdGenerator } from '@bts-soft/common';
import { DeliveryStatus } from '../enums/delivery-status.enum';
import { PaymentStatus } from '../enums/payment-status.enum';
import { DeliveryStatusHistory } from './delivery-status-history.entity';
import { DeliverySagaState } from './delivery-saga-state.entity';

@Entity({ name: 'deliveries' })
export class Delivery {
  @PrimaryColumn({ type: 'varchar', length: 20 })
  id: string = IdGenerator.generate('snowflake');

  @Index()
  @Column({ type: 'varchar', length: 20 })
  customerId!: string;

  @Index()
  @Column({ type: 'varchar', length: 20, nullable: true })
  driverId!: string | null;

  @Column({
    type: 'enum',
    enum: DeliveryStatus,
    default: DeliveryStatus.CREATED,
  })
  status!: DeliveryStatus;

  @Column({ type: 'enum', enum: PaymentStatus, default: PaymentStatus.PENDING })
  paymentStatus!: PaymentStatus;

  @Column({ type: 'decimal', precision: 12, scale: 2 })
  amount!: string;

  @Column({ type: 'varchar', length: 3, default: 'USD' })
  currency!: string;

  @Column(() => Address, { prefix: 'pickup' })
  pickupAddress!: Address;

  @Column(() => Address, { prefix: 'dropoff' })
  dropoffAddress!: Address;

  @Column({ type: 'timestamp with time zone', nullable: true })
  pickedUpAt!: Date | null;

  @Column({ type: 'timestamp with time zone', nullable: true })
  completedAt!: Date | null;

  @Column({ type: 'timestamp with time zone', nullable: true })
  cancelledAt!: Date | null;

  @OneToMany(() => DeliveryStatusHistory, (history) => history.delivery)
  statusHistory!: DeliveryStatusHistory[];

  @OneToOne(() => DeliverySagaState, (saga) => saga.delivery)
  sagaStates!: DeliverySagaState[];

  @CreateDateColumn({ type: 'timestamp with time zone' })
  createdAt!: Date;

  @UpdateDateColumn({ type: 'timestamp with time zone' })
  updatedAt!: Date;
}
