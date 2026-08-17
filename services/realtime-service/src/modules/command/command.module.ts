import { Global, Module } from '@nestjs/common';
import { CommandService } from './command.service';
import { AuthorizationModule } from '../authorization/authorization.module';

@Global()
@Module({
  imports: [AuthorizationModule],
  providers: [CommandService],
  exports: [CommandService],
})
export class CommandModule {}