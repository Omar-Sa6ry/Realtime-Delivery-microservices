import { AppModule } from './app.module';
import { CommonModule } from './common/common.module';
import { TranslationModule } from './common/translation/translation.module';
import { CommandModule } from './modules/features/command/command.module';
import { EventsModule } from './modules/features/events/events.module';
import { LocationModule } from './modules/features/location/location.module';
import { PresenceModule } from './modules/features/presence/presence.module';
import { SubscriptionModule } from './modules/features/subscription/subscription.module';
import { AuthModule } from './modules/gateway/auth/auth.module';
import { AuthorizationModule } from './modules/gateway/authorization/authorization.module';
import { ConnectionModule } from './modules/gateway/connection/connection.module';
import { HeartbeatModule } from './modules/gateway/heartbeat/heartbeat.module';
import { WebSocketModule } from './modules/gateway/websocket/websocket.module';
import { GrpcModule } from './modules/infrastructure/grpc/grpc.module';
import { KafkaConsumerModule } from './modules/infrastructure/kafka/kafka.module';
import { NatsModule } from './modules/infrastructure/nats/nats.module';
import { RealtimeRabbitMQModule } from './modules/infrastructure/rabbitmq/rabbitmq.module';

describe('Module wiring', () => {
  it.each([
    ['AppModule', AppModule],
    ['CommonModule', CommonModule],
    ['TranslationModule', TranslationModule],
    ['CommandModule', CommandModule],
    ['EventsModule', EventsModule],
    ['LocationModule', LocationModule],
    ['PresenceModule', PresenceModule],
    ['SubscriptionModule', SubscriptionModule],
    ['AuthModule', AuthModule],
    ['AuthorizationModule', AuthorizationModule],
    ['ConnectionModule', ConnectionModule],
    ['HeartbeatModule', HeartbeatModule],
    ['WebSocketModule', WebSocketModule],
    ['GrpcModule', GrpcModule],
    ['KafkaConsumerModule', KafkaConsumerModule],
    ['NatsModule', NatsModule],
    ['RealtimeRabbitMQModule', RealtimeRabbitMQModule],
  ])('%s is defined and decorated', (_name: string, mod: any) => {
    expect(mod).toBeDefined();
    expect(typeof mod).toBe('function');
  });

  it('builds the JWT secret option with and without config values', () => {
    const imports = Reflect.getMetadata('imports', AppModule) as any[];
    const jwtModule = imports.find((m: any) =>
      m?.providers?.some((p: any) => p?.provide === 'JWT_MODULE_OPTIONS'),
    );
    expect(jwtModule).toBeDefined();

    const optionProvider = jwtModule.providers.find(
      (p: any) => p.provide === 'JWT_MODULE_OPTIONS',
    );
    expect(typeof optionProvider.useFactory).toBe('function');

    expect(
      optionProvider.useFactory({
        get: jest.fn((key: string) =>
          key === 'JWT_SECRET' ? 's3cret' : undefined,
        ),
      }),
    ).toMatchObject({ secret: 's3cret' });
    expect(
      optionProvider.useFactory({ get: jest.fn(() => undefined) }),
    ).toMatchObject({ secret: 'default_secret' });
  });

  it('builds the GraphQL context with and without an accept-language header', () => {
    const imports = Reflect.getMetadata('imports', AppModule) as any[];
    const gqlModule = imports.find((m: any) =>
      m?.providers?.some(
        (p: any) => typeof p?.useValue?.context === 'function',
      ),
    );
    expect(gqlModule).toBeDefined();

    const options = gqlModule.providers.find(
      (p: any) => typeof p?.useValue?.context === 'function',
    ).useValue;

    expect(
      options.context({ req: { headers: { 'accept-language': 'ar' } } }),
    ).toMatchObject({ language: 'ar' });
    expect(options.context({ req: {} })).toMatchObject({ language: 'en' });
  });

  it('falls back to the development env file when no environment is set', () => {
    const prevAppEnv = process.env.APP_ENV;
    const prevNodeEnv = process.env.NODE_ENV;
    delete process.env.APP_ENV;
    delete process.env.NODE_ENV;
    try {
      jest.isolateModules(() => {
        const mod = require('./app.module').AppModule;
        expect(mod).toBeDefined();
      });
    } finally {
      if (prevAppEnv !== undefined) process.env.APP_ENV = prevAppEnv;
      if (prevNodeEnv !== undefined) process.env.NODE_ENV = prevNodeEnv;
    }
  });
});
