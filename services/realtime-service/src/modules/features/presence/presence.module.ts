import { Global, Module } from '@nestjs/common';
import { PresenceService } from './presence.service';
import { PresenceStore } from './presence.store';

@Global()
@Module({
  providers: [PresenceService, PresenceStore],
  exports: [PresenceService, PresenceStore],
})
export class PresenceModule {}