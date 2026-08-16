import {
  Entity,
  PrimaryColumn,
  Column,
  CreateDateColumn,
  UpdateDateColumn,
  ManyToOne,
  JoinColumn,
  Index,
} from 'typeorm';
import { NotificationChannel, DeliveryChannelStatus } from '@delivery/common';
import { IdGenerator } from '@bts-soft/common';
import { Notification } from './notification.entity';

@Entity('notification_deliveries')
export class NotificationDelivery {
  @PrimaryColumn({ type: 'varchar', length: 64 })
  id: string = IdGenerator.generate('snowflake');

  @Column()
  @Index()
  notificationId: string;

  @ManyToOne(() => Notification, (notification) => notification.deliveries, { onDelete: 'CASCADE' })
  @JoinColumn({ name: 'notificationId' })
  notification: Notification;

  @Column({ type: 'enum', enum: NotificationChannel })
  channel: NotificationChannel;

  @Column({
    type: 'enum',
    enum: DeliveryChannelStatus,
    default: DeliveryChannelStatus.PENDING,
  })
  status: DeliveryChannelStatus;

  @Column({ type: 'varchar', nullable: true })
  provider: string | null;

  @Column({ type: 'varchar', nullable: true })
  providerMessageId: string | null;

  @Column({ default: 0 })
  attemptCount: number;

  @Column({ type: 'text', nullable: true })
  lastError: string | null;

  @Column({ type: 'timestamp', nullable: true })
  scheduledAt: Date | null;

  @Column({ type: 'timestamp', nullable: true })
  sentAt: Date | null;

  @Column({ type: 'timestamp', nullable: true })
  deliveredAt: Date | null;

  @Column({ type: 'timestamp', nullable: true })
  failedAt: Date | null;

  @CreateDateColumn()
  createdAt: Date;

  @UpdateDateColumn()
  updatedAt: Date;
}