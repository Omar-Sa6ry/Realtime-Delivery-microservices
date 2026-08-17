import { Global, Module } from '@nestjs/common';
import { ScheduleModule } from '@nestjs/schedule';
import { HeartbeatService } from './heartbeat.service';
import { ConnectionModule } from '../connection/connection.module';

@Global()
@Module({
  imports: [ScheduleModule.forRoot(), ConnectionModule],
  providers: [HeartbeatService],
  exports: [HeartbeatService],
})
export class HeartbeatModule {}