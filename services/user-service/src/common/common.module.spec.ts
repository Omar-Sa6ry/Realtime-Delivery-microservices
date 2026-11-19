import { GLOBAL_MODULE_METADATA } from '@nestjs/common/constants';
import { CommonModule } from './common.module';
import { BcryptPasswordHasher } from './security/bcrypt-password.hasher';
import { JwtTokenProvider } from './security/jwt-token.provider';
import { RedisSessionRepository } from './database/repositories/redis-session.repository';
import { OutboxWorkerService } from './messaging/outbox-worker.service';
import { OutboxProcessor } from './messaging/outbox.queue';
import { GraphQLResponseInterceptor } from '@delivery-micro/shard';

describe('CommonModule', () => {
  const metadata = (key: string): any[] =>
    (Reflect.getMetadata(key, CommonModule) as any[]) ?? [];

  /** Class providers resolve to their class name, object providers to `provide`. */
  const providerTokens = () =>
    metadata('providers').map((p) => p.provide ?? p?.name ?? p);

  const exportTokens = () =>
    metadata('exports').map((e) => e.provide ?? e?.name ?? e);

  const buildingBlocks = [
    BcryptPasswordHasher.name,
    JwtTokenProvider.name,
    RedisSessionRepository.name,
    OutboxWorkerService.name,
    OutboxProcessor.name,
    GraphQLResponseInterceptor.name,
    'SHARED_REDIS_SERVICE',
  ];

  it('is registered as a global module', () => {
    expect(Reflect.getMetadata(GLOBAL_MODULE_METADATA, CommonModule)).toBe(
      true,
    );
  });

  it('registers the feature repositories for the shared entities', () => {
    const typeOrm = metadata('imports').find(
      (m) => m?.module?.name === 'TypeOrmModule',
    );

    expect(typeOrm).toBeDefined();
    expect(typeOrm.providers).toBeDefined();
  });

  it('registers the outbox BullMQ queue', () => {
    const bull = metadata('imports').find(
      (m) => m?.module?.name === 'BullModule',
    );

    expect(bull).toBeDefined();
    expect(bull.providers).toBeDefined();
  });

  it('provides the security, session and outbox building blocks', () => {
    expect(providerTokens()).toEqual(expect.arrayContaining(buildingBlocks));
  });

  it('re-exports every building block so consumers do not re-declare them', () => {
    expect(exportTokens()).toEqual(expect.arrayContaining(buildingBlocks));
  });

  it('re-exports the shared infrastructure modules', () => {
    expect(exportTokens()).toEqual(
      expect.arrayContaining([
        'ConfigModule',
        'RedisModule',
        'NotificationModule',
        'TypeOrmModule',
        'BullModule',
      ]),
    );
  });

  it('aliases the shared redis service onto the bts RedisService token', () => {
    const alias = metadata('providers').find(
      (p) => p.provide === 'SHARED_REDIS_SERVICE',
    );

    expect(alias.useExisting).toBeDefined();
    expect(alias.useValue).toBeUndefined();
  });
});
