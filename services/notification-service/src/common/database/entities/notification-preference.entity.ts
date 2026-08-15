import {
  Entity,
  PrimaryColumn,
  Column,
  CreateDateColumn,
  UpdateDateColumn,
  Index,
  Unique,
} from 'typeorm';
import { NotificationType, NotificationChannel } from '@delivery/common';
import { IdGenerator } from '@bts-soft/common';

@Entity('notification_preferences')
@Unique(['userId', 'type', 'channel'])
export class NotificationPreference {
  @PrimaryColumn({ type: 'varchar', length: 64 })
  id: string = IdGenerator.generate('snowflake');

  @Column()
  @Index()
  userId: string;

  @Column({ type: 'enum', enum: NotificationType })
  type: NotificationType;

  @Column({ type: 'enum', enum: NotificationChannel })
  channel: NotificationChannel;

  @Column({ default: true })
  enabled: boolean;

  @CreateDateColumn()
  createdAt: Date;

  @UpdateDateColumn()
  updatedAt: Date;
}