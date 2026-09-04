import {
  Column,
  CreateDateColumn,
  Entity,
  Index,
  PrimaryColumn,
  UpdateDateColumn,
} from 'typeorm';
import { IdGenerator } from '@bts-soft/common';

export enum OutboxStatus {
  PENDING = 'PENDING',
  PROCESSING = 'PROCESSING',
  PUBLISHED = 'PUBLISHED',
  FAILED = 'FAILED',
}

@Entity({ name: 'outbox_events' })
export class Outbox {
  @PrimaryColumn({ type: 'varchar', length: 20 })
  id: string = IdGenerator.generate('snowflake');

  @Index({ unique: true })
  @Column({ type: 'varchar', length: 64 })
  eventId!: string;

  @Index()
  @Column({ type: 'varchar', length: 120 })
  eventType!: string;

  @Index()
  @Column({ type: 'varchar', length: 120 })
  aggregateId!: string;

  @Column({ type: 'varchar', length: 80, default: 'delivery' })
  aggregateType!: string;

  @Column({ type: 'jsonb' })
  payload!: Record<string, unknown>;

  @Column({ type: 'enum', enum: OutboxStatus, default: OutboxStatus.PENDING })
  status!: OutboxStatus;

  @Column({ type: 'int', default: 0 })
  attempts!: number;

  @Column({ type: 'timestamp with time zone', nullable: true })
  availableAt!: Date | null;

  @Column({ type: 'timestamp with time zone', nullable: true })
  publishedAt!: Date | null;

  @Column({ type: 'text', nullable: true })
  lastError!: string | null;

  @CreateDateColumn({ type: 'timestamp with time zone' })
  createdAt!: Date;

  @UpdateDateColumn({ type: 'timestamp with time zone' })
  updatedAt!: Date;
}
