import {
  Column,
  CreateDateColumn,
  Entity,
  Index,
  JoinColumn,
  OneToOne,
  PrimaryColumn,
  UpdateDateColumn,
} from 'typeorm';
import { Delivery } from './delivery.entity';
import { IdGenerator } from '@bts-soft/common';

@Entity({ name: 'delivery_saga_states' })
export class DeliverySagaState {
  @PrimaryColumn({ type: 'varchar', length: 20 })
  id: string = IdGenerator.generate('snowflake');

  @Index({ unique: true })
  @Column({ type: 'varchar', length: 20 })
  deliveryId!: string;

  @Column({ type: 'varchar', length: 80, default: 'CREATE_DELIVERY' })
  currentStep!: string;

  @Column({ type: 'varchar', length: 30, default: 'RUNNING' })
  status!: string;

  @Column({ type: 'jsonb', default: {} })
  context!: Record<string, unknown>;

  @Column({ type: 'int', default: 1 })
  version!: number;

  @OneToOne(() => Delivery, (delivery) => delivery.sagaState, {
    onDelete: 'CASCADE',
  })
  @JoinColumn({ name: 'deliveryId' })
  delivery!: Delivery;

  @CreateDateColumn({ type: 'timestamp with time zone' })
  createdAt!: Date;

  @UpdateDateColumn({ type: 'timestamp with time zone' })
  updatedAt!: Date;
}
