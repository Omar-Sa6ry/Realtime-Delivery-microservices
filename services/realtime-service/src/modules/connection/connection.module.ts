import { Global, Module } from '@nestjs/common';
import { ConnectionService } from './connection.service';
import { ConnectionRegistry } from './connection.registry';
import { ConnectionStateStore } from './connection-state.store';
import { SocketWriter } from './socket-writer.service';
import { PresenceModule } from '../presence/presence.module';

@Global()
@Module({
  imports: [PresenceModule],
  providers: [
    ConnectionService,
    ConnectionRegistry,
    ConnectionStateStore,
    SocketWriter,
  ],
  exports: [ConnectionService, ConnectionRegistry, ConnectionStateStore, SocketWriter],
})
export class ConnectionModule {}