import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import { MicroserviceOptions, Transport } from '@nestjs/microservices';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { join } from 'path';
import { NotificationService } from '../src/modules/notification/notification.service';
import { GrpcController } from '../src/modules/grpc/grpc.controller';
import { NotificationType } from '@delivery-micro/shard';

jest.setTimeout(30000);

const GRPC_PORT = 50153;
const PROTO_PATH = join(process.cwd(), '../../protos/notification.proto');

type GrpcClient = {
  SendNotification: (
    req: Record<string, unknown>,
    callback: (err: Error | null, res: any) => void,
  ) => void;
};

describe('Notification gRPC (e2e)', () => {
  let app: INestApplication;
  let client: GrpcClient;
  let notificationService: { createAndDispatch: jest.Mock };

  const waitForServer = async (retries = 40): Promise<void> => {
    for (let i = 0; i < retries; i += 1) {
      try {
        await new Promise<void>((resolve, reject) => {
          client.SendNotification(
            {
              userId: 'ping',
              type: 'x',
              title: 't',
              body: 'b',
              data: '',
              priority: 'NORMAL',
            },
            (err) => (err ? reject(err) : resolve()),
          );
        });
        return;
      } catch {
        await new Promise((r) => setTimeout(r, 250));
      }
    }
    throw new Error('gRPC server did not become ready');
  };

  const sendNotification = (
    payload: Record<string, unknown>,
  ): Promise<{ notificationId: string; success: boolean }> =>
    new Promise((resolve, reject) => {
      client.SendNotification(payload, (err, res) => {
        if (err) reject(err);
        else resolve(res);
      });
    });

  beforeAll(async () => {
    notificationService = { createAndDispatch: jest.fn() };

    const moduleFixture: TestingModule = await Test.createTestingModule({
      controllers: [GrpcController],
      providers: [
        {
          provide: NotificationService,
          useValue: notificationService,
        },
      ],
    }).compile();

    app = moduleFixture.createNestApplication();
    app.connectMicroservice<MicroserviceOptions>({
      transport: Transport.GRPC,
      options: {
        package: 'notification',
        protoPath: PROTO_PATH,
        url: `0.0.0.0:${GRPC_PORT}`,
      },
    });
    await app.startAllMicroservices();
    await app.init();

    const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
      keepCase: false,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    });
    const proto = grpc.loadPackageDefinition(packageDefinition) as any;
    client = new proto.notification.NotificationService(
      `127.0.0.1:${GRPC_PORT}`,
      grpc.credentials.createInsecure(),
    );
    await waitForServer();
  });

  afterAll(async () => {
    client?.close?.();
    await app.close();
  });

  beforeEach(() => {
    notificationService.createAndDispatch.mockReset();
  });

  it('creates a notification from valid JSON data', async () => {
    notificationService.createAndDispatch.mockResolvedValue('n-grpc-1');

    const res = await sendNotification({
      userId: 'user-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 'Order update',
      body: 'Your driver is on the way',
      data: JSON.stringify({ orderId: 'o-9' }),
      priority: 'HIGH',
    });

    expect(res.notificationId).toBe('n-grpc-1');
    expect(res.success).toBe(true);
    expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
      expect.objectContaining({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        title: 'Order update',
        body: 'Your driver is on the way',
        data: { orderId: 'o-9' },
        priority: 'HIGH',
      }),
    );
  });

  it('survives malformed JSON payloads', async () => {
    notificationService.createAndDispatch.mockResolvedValue('n-grpc-2');

    const res = await sendNotification({
      userId: 'user-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 't',
      body: 'b',
      data: '{broken-json',
      priority: 'NORMAL',
    });

    expect(res.success).toBe(true);
    expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
      expect.objectContaining({ data: {} }),
    );
  });

  it('returns success: false when creation fails', async () => {
    notificationService.createAndDispatch.mockResolvedValue(null);

    const res = await sendNotification({
      userId: 'user-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 't',
      body: 'b',
      data: '',
      priority: 'NORMAL',
    });

    expect(res.success).toBe(false);
    expect(res.notificationId).toBeFalsy();
  });
});
