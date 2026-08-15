import {
  Entity,
  PrimaryColumn,
  Column,
  CreateDateColumn,
  UpdateDateColumn,
  Unique,
} from 'typeorm';
import { NotificationType, NotificationChannel } from '@delivery/common';
import { IdGenerator } from '@bts-soft/common';

@Entity('notification_templates')
@Unique(['type', 'channel', 'locale'])
export class NotificationTemplate {
  @PrimaryColumn({ type: 'varchar', length: 64 })
  id: string = IdGenerator.generate('snowflake');

  @Column({ type: 'enum', enum: NotificationType })
  type: NotificationType;

  @Column({ type: 'enum', enum: NotificationChannel })
  channel: NotificationChannel;

  @Column({ length: 10, default: 'en' })
  locale: string;

  @Column()
  titleTemplate: string;

  @Column('text')
  bodyTemplate: string;

  @CreateDateColumn()
  createdAt: Date;

  @UpdateDateColumn()
  updatedAt: Date;
}