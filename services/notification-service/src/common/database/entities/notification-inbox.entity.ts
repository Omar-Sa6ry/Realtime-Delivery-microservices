import {
  Entity,
  PrimaryColumn,
  Column,
  CreateDateColumn,
  Unique,
} from 'typeorm';
import { IdGenerator } from '@bts-soft/common';

@Entity('notification_inbox')
@Unique(['eventId', 'consumer'])
export class NotificationInbox {
  @PrimaryColumn({ type: 'varchar', length: 64 })
  id: string = IdGenerator.generate('snowflake');

  @Column()
  eventId: string;

  @Column()
  eventType: string;

  @Column({ default: 'notification-service' })
  consumer: string;

  @Column({ type: 'timestamp', default: () => 'CURRENT_TIMESTAMP' })
  processedAt: Date;

  @CreateDateColumn()
  createdAt: Date;
}