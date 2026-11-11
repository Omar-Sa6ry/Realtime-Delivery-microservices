import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import { DataSource } from 'typeorm';
import { getRepositoryToken } from '@nestjs/typeorm';
import { getQueueToken } from '@nestjs/bullmq';
import { I18nService } from 'nestjs-i18n';
import {
  ApolloFederationDriver,
  ApolloFederationDriverConfig,
} from '@nestjs/apollo';
import { GraphQLModule } from '@nestjs/graphql';
import { JwtModule, JwtService } from '@nestjs/jwt';
import request from 'supertest';
import { App } from 'supertest/types';
import { RedisService } from '@bts-soft/cache';
import { NotificationService as BtsNotificationService } from '@bts-soft/notifications';

import { AppResolver } from '../src/app.resolver';
import { NotificationResolver } from '../src/modules/notification/notification.resolver';
import { NotificationService } from '../src/modules/notification/notification.service';
import { NotificationDispatcherService } from '../src/modules/notification/notification-dispatcher.service';
import { TemplateService } from '../src/modules/notification/template/template.service';
import { PreferenceService } from '../src/modules/notification/preference/preference.service';
import { EmailWorker } from '../src/modules/workers/email.worker';
import { DeliveryStateService } from '../src/modules/workers/delivery-state.service';
import { Notification } from '../src/common/database/entities/notification.entity';
import { NotificationDelivery } from '../src/common/database/entities/notification-delivery.entity';
import { NotificationOutbox } from '../src/common/database/entities/notification-outbox.entity';
import { NotificationPreference } from '../src/common/database/entities/notification-preference.entity';
import { NotificationTemplate } from '../src/common/database/entities/notification-template.entity';
import {
  DeliveryChannelStatus,
  NotificationChannel,
  NotificationPriority,
  NotificationStatus,
  NotificationType,
} from '@delivery-micro/shard';
import { ChannelType } from '@bts-soft/notifications';

const JWT_SECRET = 'e2e-test-secret';

jest.setTimeout(30000);

describe('Notification GraphQL + flow (e2e)', () => {
  let app: INestApplication<App>;
  let token: string;
  let notificationRepository: any;
  let deliveryRepository: any;
  let outboxRepository: any;
  let preferenceRepository: any;
  let templateRepository: any;
  let queues: Record<string, { add: jest.Mock; upsertJobScheduler: jest.Mock }>;
  let queryRunner: any;
  let btsSend: jest.Mock;

  const storedNotification = (): Notification =>
    ({
      id: 'n-1',
      userId: 'user-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 'Order created',
      body: 'Your order is on the way',
      data: { orderId: 'o-1' },
      priority: NotificationPriority.NORMAL,
      status: NotificationStatus.CREATED,
      scheduledAt: null,
      expiresAt: null,
      readAt: null,
      createdAt: new Date('2026-01-01T00:00:00Z'),
      updatedAt: new Date('2026-01-01T00:00:00Z'),
      deliveries: [],
    }) as Notification;

  const gql = (query: string, variables?: Record<string, unknown>) =>
    request(app.getHttpServer())
      .post('/notification/graphql')
      .set('Authorization', `Bearer ${token}`)
      .send({ query, variables });

  beforeAll(() => {
    process.env.JWT_SECRET = JWT_SECRET;
  });

  beforeEach(async () => {
    notificationRepository = {
      find: jest.fn().mockResolvedValue([]),
      findOne: jest.fn().mockResolvedValue(null),
      findAndCount: jest.fn().mockResolvedValue([[], 0]),
      create: jest.fn().mockImplementation((_e: any, data: any) => ({
        ...data,
        id: `created-${Math.random().toString(36).slice(2, 8)}`,
      })),
      save: jest.fn().mockImplementation(async (x: any) => x),
      update: jest.fn().mockResolvedValue({ affected: 1 }),
      delete: jest.fn().mockResolvedValue({ affected: 1 }),
      count: jest.fn().mockResolvedValue(0),
    };
    deliveryRepository = {
      find: jest.fn().mockResolvedValue([]),
      findOne: jest.fn().mockResolvedValue(null),
      create: jest
        .fn()
        .mockImplementation((_e: any, data: any) => ({ ...data, id: 'd-new' })),
      save: jest.fn().mockImplementation(async (x: any) => x),
    };
    outboxRepository = {
      findOne: jest.fn().mockResolvedValue(null),
      create: jest.fn().mockImplementation((data: any) => ({
        ...data,
        id: 'out-1',
      })),
      save: jest.fn().mockImplementation(async (x: any) => x),
    };
    preferenceRepository = {
      find: jest.fn().mockResolvedValue([]),
      findOne: jest.fn().mockResolvedValue(null),
      create: jest.fn().mockImplementation((data: any) => ({
        ...data,
        id: 'pref-new',
      })),
      save: jest.fn().mockImplementation(async (x: any) => ({
        ...x,
        id: x.id ?? 'pref-new',
      })),
    };
    templateRepository = {
      find: jest.fn().mockResolvedValue([
        {
          id: 'tpl-1',
          type: NotificationType.DELIVERY_CREATED,
          channel: NotificationChannel.IN_APP,
          locale: 'en',
          titleTemplate: 'Order {{orderId}} created',
          bodyTemplate: 'Hello {{firstName}}, order {{orderId}} is ready.',
        },
      ]),
      findOne: jest.fn().mockResolvedValue(null),
      create: jest.fn().mockImplementation((data: any) => data),
      save: jest.fn().mockImplementation(async (x: any) => x),
    };

    const queueFactory = () => ({
      add: jest.fn().mockResolvedValue({ id: 'job-1' }),
      upsertJobScheduler: jest.fn().mockResolvedValue(undefined),
    });
    queues = {
      'notification-email': queueFactory(),
      'notification-sms': queueFactory(),
      'notification-push': queueFactory(),
      'notification-inapp': queueFactory(),
      'notification-realtime': queueFactory(),
      'notification-scheduled': queueFactory(),
      'notification-retry': queueFactory(),
    };

    queryRunner = {
      connect: jest.fn().mockResolvedValue(undefined),
      startTransaction: jest.fn().mockResolvedValue(undefined),
      commitTransaction: jest.fn().mockResolvedValue(undefined),
      rollbackTransaction: jest.fn().mockResolvedValue(undefined),
      release: jest.fn().mockResolvedValue(undefined),
      manager: {
        create: jest.fn().mockImplementation((_e: any, data: any) => ({
          ...data,
          id: data?.id ?? `tx-${queryRunner.manager.create.mock.calls.length}`,
        })),
        save: jest.fn().mockImplementation(async (x: any) => x),
      },
    };

    btsSend = jest.fn().mockResolvedValue(undefined);

    const moduleFixture: TestingModule = await Test.createTestingModule({
      imports: [
        GraphQLModule.forRoot<ApolloFederationDriverConfig>({
          driver: ApolloFederationDriver,
          path: '/notification/graphql',
          autoSchemaFile: { federation: 2 },
          context: ({ req }: { req: unknown }) => ({ req }),
          playground: false,
          debug: false,
        }),
        JwtModule.register({
          secret: JWT_SECRET,
          signOptions: { expiresIn: '1h' },
        }),
      ],
      providers: [
        AppResolver,
        NotificationResolver,
        NotificationService,
        NotificationDispatcherService,
        TemplateService,
        PreferenceService,
        DeliveryStateService,
        EmailWorker,
        {
          provide: 'USER_SERVICE',
          useValue: { findById: jest.fn() },
        },
        {
          provide: I18nService,
          useValue: {
            t: jest.fn().mockImplementation(async (key: string) => key),
          },
        },
        {
          provide: DataSource,
          useValue: {
            createQueryRunner: jest.fn(() => queryRunner),
          },
        },
        {
          provide: RedisService,
          useValue: {
            get: jest.fn().mockResolvedValue(null),
            set: jest.fn().mockResolvedValue(undefined),
            del: jest.fn().mockResolvedValue(undefined),
          },
        },
        {
          provide: 'SHARED_REDIS_SERVICE',
          useValue: {},
        },
        {
          provide: BtsNotificationService,
          useValue: { send: btsSend },
        },
        {
          provide: getRepositoryToken(Notification),
          useValue: notificationRepository,
        },
        {
          provide: getRepositoryToken(NotificationDelivery),
          useValue: deliveryRepository,
        },
        {
          provide: getRepositoryToken(NotificationOutbox),
          useValue: outboxRepository,
        },
        {
          provide: getRepositoryToken(NotificationPreference),
          useValue: preferenceRepository,
        },
        {
          provide: getRepositoryToken(NotificationTemplate),
          useValue: templateRepository,
        },
        ...Object.entries(queues).map(([name, queue]) => ({
          provide: getQueueToken(name),
          useValue: queue,
        })),
      ],
    }).compile();

    app = moduleFixture.createNestApplication({ logger: false });
    await app.init();

    const jwtService = moduleFixture.get(JwtService, { strict: false });
    token = jwtService.sign({
      sub: 'user-1',
      role: 'user',
      email: 'u@t.com',
    });
  });

  afterEach(async () => {
    await app.close();
  });

  describe('AppResolver ping', () => {
    it('answers pingForNotification without auth', async () => {
      const res = await request(app.getHttpServer())
        .post('/notification/graphql')
        .send({
          query: `{ pingForNotification { success statusCode message data } }`,
        })
        .expect(200);

      expect(res.body.data.pingForNotification).toMatchObject({
        success: true,
        statusCode: 200,
        data: true,
      });
    });
  });

  describe('myNotifications', () => {
    it('returns a paginated list for the authenticated user', async () => {
      notificationRepository.findAndCount.mockResolvedValue([
        [storedNotification()],
        1,
      ]);

      const res = await gql(`
        query {
          myNotifications(page: 1, limit: 10) {
            success
            statusCode
            message
            data {
              totalCount
              items { id title body status priority }
            }
          }
        }
      `).expect(200);

      expect(res.body.errors).toBeUndefined();
      const payload = res.body.data.myNotifications;
      expect(payload.success).toBe(true);
      expect(payload.statusCode).toBe(200);
      expect(payload.message).toBe('notification.LIST_RETRIEVED');
      expect(payload.data.totalCount).toBe(1);
      expect(payload.data.items[0]).toMatchObject({
        id: 'n-1',
        title: 'Order created',
        body: 'Your order is on the way',
        status: NotificationStatus.CREATED,
        priority: NotificationPriority.NORMAL,
      });
      expect(notificationRepository.findAndCount).toHaveBeenCalledWith(
        expect.objectContaining({
          where: { userId: 'user-1' },
          relations: { deliveries: true },
        }),
      );
    });
  });

  describe('notification + unread count + mutations', () => {
    it('fetches a single notification', async () => {
      notificationRepository.findOne.mockResolvedValue(storedNotification());

      const res = await gql(`
        query {
          notification(id: "n-1") {
            success
            data { id title }
          }
        }
      `).expect(200);

      expect(res.body.data.notification.success).toBe(true);
      expect(res.body.data.notification.data.id).toBe('n-1');
    });

    it('returns the unread count', async () => {
      notificationRepository.count.mockResolvedValue(4);

      const res = await gql(`
        query { unreadNotificationCount { success data } }
      `).expect(200);

      expect(res.body.data.unreadNotificationCount.data).toBe(4);
    });

    it('marks a notification as read', async () => {
      const saved = {
        ...storedNotification(),
        readAt: new Date('2026-02-01T00:00:00Z'),
      };
      notificationRepository.findOne.mockResolvedValue(storedNotification());
      notificationRepository.save.mockImplementation(async (n: any) => ({
        ...n,
        readAt: saved.readAt,
      }));

      const res = await gql(`
        mutation {
          markNotificationAsRead(id: "n-1") {
            success
            data { id readAt }
          }
        }
      `).expect(200);

      expect(res.body.data.markNotificationAsRead.success).toBe(true);
      expect(res.body.data.markNotificationAsRead.data.id).toBe('n-1');
    });

    it('marks all notifications as read', async () => {
      const res = await gql(`
        mutation { markAllNotificationsAsRead { success data } }
      `).expect(200);

      expect(res.body.data.markAllNotificationsAsRead.data).toBe(true);
      expect(notificationRepository.update).toHaveBeenCalled();
    });

    it('deletes a notification', async () => {
      const res = await gql(`
        mutation { deleteNotification(id: "n-1") { success data } }
      `).expect(200);

      expect(res.body.data.deleteNotification.data).toBe(true);
      expect(notificationRepository.delete).toHaveBeenCalledWith({
        id: 'n-1',
        userId: 'user-1',
      });
    });

    it('rejects unauthenticated requests', async () => {
      await request(app.getHttpServer())
        .post('/notification/graphql')
        .send({
          query: `{ myNotifications { success } }`,
        })
        .expect(200)
        .expect((res) => {
          expect(res.body.errors).toBeDefined();
        });
    });
  });

  describe('preferences', () => {
    it('lists preferences', async () => {
      preferenceRepository.find.mockResolvedValue([
        {
          id: 'p-1',
          userId: 'user-1',
          type: NotificationType.DELIVERY_CREATED,
          channel: NotificationChannel.EMAIL,
          enabled: true,
        },
      ]);

      const res = await gql(`
        query {
          myNotificationPreferences {
            success
            items { id channel enabled }
          }
        }
      `).expect(200);

      expect(res.body.data.myNotificationPreferences.items[0]).toMatchObject({
        channel: NotificationChannel.EMAIL,
        enabled: true,
      });
    });

    it('updates preferences through upsert', async () => {
      const res = await gql(
        `
        mutation($preferences: [NotificationPreferenceInput!]!) {
          updateNotificationPreferences(preferences: $preferences) {
            success
            items { channel enabled }
          }
        }
      `,
        {
          preferences: [
            {
              type: NotificationType.DELIVERY_CREATED,
              channels: [
                { channel: NotificationChannel.EMAIL, enabled: true },
                { channel: NotificationChannel.SMS, enabled: false },
              ],
            },
          ],
        },
      ).expect(200);

      expect(res.body.data.updateNotificationPreferences.items).toHaveLength(2);
      expect(preferenceRepository.save).toHaveBeenCalledTimes(2);
    });
  });

  describe('full create -> dispatch -> worker flow', () => {
    it('creates a notification, queues jobs, then the email worker delivers it', async () => {
      const notificationService = app.get(NotificationService);
      const emailWorker = app.get(EmailWorker);

      const id = await notificationService.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        data: { orderId: 'o-42', firstName: 'Nour' },
        requiredChannels: [
          NotificationChannel.IN_APP,
          NotificationChannel.EMAIL,
        ],
      });

      expect(id).toBeTruthy();
      expect(queryRunner.commitTransaction).toHaveBeenCalled();
      expect(queues['notification-inapp'].add).toHaveBeenCalledWith(
        'send',
        expect.objectContaining({ notificationId: id }),
        expect.objectContaining({ jobId: `${id}-IN_APP` }),
      );
      expect(queues['notification-email'].add).toHaveBeenCalledWith(
        'send',
        expect.objectContaining({ notificationId: id }),
        expect.objectContaining({ jobId: `${id}-EMAIL` }),
      );
      expect(outboxRepository.findOne).not.toHaveBeenCalled();

      const createdNotification = queryRunner.manager.create.mock.calls.find(
        ([entity]: [any]) => entity.name === 'Notification',
      )?.[1];
      expect(createdNotification.title).toBe('Order o-42 created');
      expect(createdNotification.body).toBe('Hello Nour, order o-42 is ready.');

      // Now run the email worker against the same mocked persistence layer.
      deliveryRepository.findOne.mockResolvedValue({
        id: 'd-email',
        notificationId: id,
        channel: NotificationChannel.EMAIL,
        status: DeliveryChannelStatus.PENDING,
        attemptCount: 0,
      });
      notificationRepository.findOne.mockResolvedValue({
        ...createdNotification,
        id,
      });
      const deliveryState = {
        beginProcessing: jest.fn(),
        complete: jest.fn(),
        fail: jest.fn(),
      };
      const worker = new EmailWorker(
        notificationRepository,
        deliveryRepository,
        deliveryState as any,
        { send: btsSend } as any,
      );

      await worker.process({
        data: { notificationId: id, deliveryId: 'd-email' },
      } as any);

      expect(btsSend).toHaveBeenCalledWith(
        ChannelType.EMAIL,
        expect.objectContaining({
          recipientId: 'user-1',
          title: 'Order o-42 created',
          idempotencyKey: `email:${id}:d-email`,
        }),
      );
      expect(deliveryState.complete).toHaveBeenCalled();
    });

    it('creates a realtime outbox row when the REALTIME channel is used', async () => {
      const notificationService = app.get(NotificationService);

      const id = await notificationService.createAndDispatch({
        userId: 'user-1',
        type: NotificationType.DELIVERY_CREATED,
        requiredChannels: [NotificationChannel.REALTIME],
      });

      expect(queues['notification-realtime'].add).toHaveBeenCalled();
      expect(outboxRepository.create).toHaveBeenCalledWith(
        expect.objectContaining({
          eventType: 'NOTIFICATION_CREATED',
          aggregateId: id,
        }),
      );
      expect(outboxRepository.save).toHaveBeenCalled();
    });
  });
});
