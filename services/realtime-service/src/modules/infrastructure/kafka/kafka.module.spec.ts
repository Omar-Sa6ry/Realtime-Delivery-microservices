import { KafkaConsumerModule } from './kafka.module';
import { REALTIME_EVENT_HANDLERS } from './handlers/base-kafka-event.handler';

describe('KafkaConsumerModule wiring', () => {
  const stubs = [
    'deliveryCreated',
    'driverAssigned',
    'driverAccepted',
    'deliveryPickedUp',
    'deliveryInTransit',
    'deliveryCompleted',
    'deliveryCancelled',
    'paymentCompleted',
    'paymentFailed',
    'driverAssignmentOffered',
    'driverAssignmentRejected',
    'driverAssignmentExpired',
    'driverNoDriverAvailable',
  ].map((name) => ({ name }));

  it('aggregates every realtime event handler', () => {
    const providers: any[] = Reflect.getMetadata(
      'providers',
      KafkaConsumerModule,
    );
    const provider = providers.find(
      (p) => p?.provide === REALTIME_EVENT_HANDLERS,
    );

    expect(provider).toBeDefined();
    expect(provider.inject).toHaveLength(13);
    expect(provider.useFactory(...stubs)).toEqual(stubs);
  });

  const findKafkaFactory = () => {
    const importsMeta: any[] = Reflect.getMetadata(
      'imports',
      KafkaConsumerModule,
    );
    const dynamic = importsMeta.find((m) => m?.module && m?.providers);
    expect(dynamic).toBeDefined();
    return dynamic.providers[0].useFactory;
  };

  it('builds kafka client options from config defaults', async () => {
    const kafkaFactory = findKafkaFactory();
    const cfg = { get: jest.fn((_key: string, def: unknown) => def) };

    const svc = await kafkaFactory(cfg);
    expect(svc.options).toEqual({
      clientId: 'realtime-service',
      brokers: ['localhost:9092'],
    });
  });

  it('trims and filters configured brokers', async () => {
    const kafkaFactory = findKafkaFactory();
    const cfg = {
      get: jest.fn((key: string, def: unknown) =>
        key === 'KAFKA_BROKERS' ? 'a:9092, b:9092 ,,,' : def,
      ),
    };

    const svc = await kafkaFactory(cfg);
    expect(svc.options).toEqual({
      clientId: 'realtime-service',
      brokers: ['a:9092', 'b:9092'],
    });
  });
});
