import {
  Entity,
  PrimaryColumn,
  Column,
  CreateDateColumn,
  UpdateDateColumn,
  OneToMany,
  Index,
} from 'typeorm';
import { NotificationType, NotificationStatus, NotificationPriority } from '@delivery/common';
import { IdGenerator } from '@bts-soft/common';
import { NotificationDelivery } from './notification-delivery.entity';

@Entity('notifications')
export class Notification {
  @PrimaryColumn({ type: 'varchar', length: 64 })
  id: string = IdGenerator.generate('snowflake');

  @Column()
  @Index()
  userId: string;

  @Column({ type: 'enum', enum: NotificationType })
  type: NotificationType;

  @Column()
  title: string;

  @Column()
  body: string;

  @Column({ type: 'jsonb', nullable: true })
  data: Record<string, unknown> | null;

  @Column({
    type: 'enum',
    enum: NotificationPriority,
    default: NotificationPriority.NORMAL,
  })
  priority: NotificationPriority;

  @Column({
    type: 'enum',
    enum: NotificationStatus,
    default: NotificationStatus.CREATED,
  })
  status: NotificationStatus;

  @Column({ type: 'timestamp', nullable: true })
  scheduledAt: Date | null;

  @Column({ type: 'timestamp', nullable: true })
  expiresAt: Date | null;

  @Column({ type: 'timestamp', nullable: true })
  readAt: Date | null;

  @CreateDateColumn()
  createdAt: Date;

  @UpdateDateColumn()
  updatedAt: Date;

  @OneToMany(() => NotificationDelivery, (delivery) => delivery.notification)
  deliveries: NotificationDelivery[];
}