import {
  Entity,
  PrimaryGeneratedColumn,
  Column,
  CreateDateColumn,
  Unique,
} from 'typeorm';

@Entity('notification_inbox')
@Unique(['eventId', 'consumer'])
export class NotificationInbox {
  @PrimaryGeneratedColumn('uuid')
  id: string;

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
