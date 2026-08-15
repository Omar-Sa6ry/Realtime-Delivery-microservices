import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { NotificationOutbox } from '../../common/database/entities/notification-outbox.entity';
import { OutboxWorkerService } from './outbox-worker.service';

@Module({
  imports: [TypeOrmModule.forFeature([NotificationOutbox])],
  providers: [OutboxWorkerService],
})
export class OutboxModule {}
