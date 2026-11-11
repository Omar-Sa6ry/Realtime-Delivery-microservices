import { TemplateService } from './template.service';
import { NotificationTemplate } from '../../../common/database/entities/notification-template.entity';
import { NotificationChannel, NotificationType } from '@delivery-micro/shard';
import { RedisService } from '@bts-soft/cache';
import * as Handlebars from 'handlebars';
import { createMockRepository, MockRepository } from '../../../test-utils';

describe('TemplateService', () => {
  let service: TemplateService;
  let repository: MockRepository<NotificationTemplate>;
  let redisService: { get: jest.Mock; set: jest.Mock };

  const templateEntity = (
    overrides: Partial<NotificationTemplate> = {},
  ): Partial<NotificationTemplate> => ({
    type: NotificationType.DELIVERY_CREATED,
    channel: NotificationChannel.IN_APP,
    locale: 'en',
    titleTemplate: 'Order {{orderId}} created',
    bodyTemplate: 'Hello {{firstName}}, your order is ready.',
    ...overrides,
  });

  beforeEach(() => {
    repository = createMockRepository<NotificationTemplate>();
    redisService = {
      get: jest.fn().mockResolvedValue(null),
      set: jest.fn().mockResolvedValue(undefined),
    };
    service = new TemplateService(
      repository as any,
      redisService as unknown as RedisService,
    );
  });

  describe('onModuleInit', () => {
    it('seeds missing default templates and loads all templates into memory', async () => {
      repository.findOne.mockResolvedValue(null);
      const seeded: any[] = [];
      repository.create.mockImplementation((data: any) => data);
      repository.save.mockImplementation(async (entity: any) => {
        seeded.push(entity);
        return entity;
      });
      repository.find.mockResolvedValue([
        templateEntity() as NotificationTemplate,
      ]);

      await service.onModuleInit();

      expect(repository.findOne).toHaveBeenCalledTimes(2);
      expect(repository.save).toHaveBeenCalledTimes(2);
      expect(seeded).toHaveLength(2);
      expect(repository.find).toHaveBeenCalled();
      expect((service as any).templatesCache.size).toBe(1);
    });

    it('skips creating templates that already exist', async () => {
      repository.findOne.mockResolvedValue({ id: 'existing' });
      repository.find.mockResolvedValue([]);

      await service.onModuleInit();

      expect(repository.create).not.toHaveBeenCalled();
      expect(repository.save).not.toHaveBeenCalled();
    });
  });

  describe('loadTemplates', () => {
    it('compiles and caches every template row', async () => {
      repository.find.mockResolvedValue([
        templateEntity() as NotificationTemplate,
        templateEntity({ locale: 'ar' }) as NotificationTemplate,
      ]);

      await service.loadTemplates();

      const cache = (service as any).templatesCache as Map<string, unknown>;
      expect(cache.size).toBe(2);
      expect(
        cache.has(
          `${NotificationType.DELIVERY_CREATED}:${NotificationChannel.IN_APP}:en`,
        ),
      ).toBe(true);
    });

    it('does not cache templates that fail to compile', async () => {
      const compileSpy = jest
        .spyOn(Handlebars, 'compile')
        .mockImplementation((() => {
          throw new Error('compile boom');
        }) as any);

      repository.find.mockResolvedValue([
        templateEntity() as NotificationTemplate,
      ]);

      await service.loadTemplates();

      const cache = (service as any).templatesCache as Map<string, unknown>;
      expect(cache.size).toBe(0);

      compileSpy.mockRestore();
    });
  });

  describe('render', () => {
    it('renders from the in-memory cache (memory hit)', async () => {
      repository.find.mockResolvedValue([
        templateEntity() as NotificationTemplate,
      ]);
      await service.loadTemplates();

      const result = await service.render(
        NotificationType.DELIVERY_CREATED,
        NotificationChannel.IN_APP,
        'en',
        { orderId: '42', firstName: 'Ali' },
      );

      expect(result.title).toBe('Order 42 created');
      expect(result.body).toBe('Hello Ali, your order is ready.');
      expect(redisService.get).not.toHaveBeenCalled();
      expect(repository.findOne).not.toHaveBeenCalled();
    });

    it('resolves template from Redis on memory miss (redis hit)', async () => {
      redisService.get.mockResolvedValue({
        type: NotificationType.DELIVERY_CREATED,
        channel: NotificationChannel.IN_APP,
        locale: 'en',
        titleTemplate: 'Redis title {{orderId}}',
        bodyTemplate: 'Redis body',
      });

      const result = await service.render(
        NotificationType.DELIVERY_CREATED,
        NotificationChannel.IN_APP,
        'en',
        { orderId: '7' },
      );

      expect(redisService.get).toHaveBeenCalledWith(
        `template:${NotificationType.DELIVERY_CREATED}:${NotificationChannel.IN_APP}:en`,
      );
      expect(result.title).toBe('Redis title 7');
      expect(result.body).toBe('Redis body');
      expect(repository.findOne).not.toHaveBeenCalled();
    });

    it('loads from database, stores in redis and memory on full cache miss', async () => {
      repository.findOne.mockResolvedValue(
        templateEntity() as NotificationTemplate,
      );

      const result = await service.render(
        NotificationType.DELIVERY_CREATED,
        NotificationChannel.IN_APP,
        'en',
        { orderId: '9', firstName: 'Sara' },
      );

      expect(repository.findOne).toHaveBeenCalledWith({
        where: {
          type: NotificationType.DELIVERY_CREATED,
          channel: NotificationChannel.IN_APP,
          locale: 'en',
        },
      });
      expect(redisService.set).toHaveBeenCalledWith(
        `template:${NotificationType.DELIVERY_CREATED}:${NotificationChannel.IN_APP}:en`,
        expect.objectContaining({ titleTemplate: 'Order {{orderId}} created' }),
        3600,
      );
      expect(result.title).toBe('Order 9 created');
    });

    it('falls back to the en locale when requested locale is missing', async () => {
      repository.findOne.mockImplementation(async (options: any) => {
        if (options.where.locale === 'en') {
          return templateEntity() as NotificationTemplate;
        }
        return null;
      });

      const result = await service.render(
        NotificationType.DELIVERY_CREATED,
        NotificationChannel.IN_APP,
        'ar',
        { orderId: '1', firstName: 'Omar' },
      );

      expect(result.title).toBe('Order 1 created');
      expect(repository.findOne).toHaveBeenCalledWith({
        where: {
          type: NotificationType.DELIVERY_CREATED,
          channel: NotificationChannel.IN_APP,
          locale: 'ar',
        },
      });
    });

    it('falls back to default format when no template is found at all', async () => {
      repository.findOne.mockResolvedValue(null);

      const result = await service.render(
        NotificationType.PAYMENT_FAILED,
        NotificationChannel.EMAIL,
        'fr',
        { amount: 10 },
      );

      expect(result.title).toBe(
        `Notification: ${NotificationType.PAYMENT_FAILED}`,
      );
      expect(result.body).toBe(JSON.stringify({ amount: 10 }));
    });

    it('falls back to default format when rendering throws', async () => {
      (service as any).templatesCache.set(
        `${NotificationType.DELIVERY_CREATED}:${NotificationChannel.IN_APP}:en`,
        {
          title: () => {
            throw new Error('render boom');
          },
          body: () => 'never',
        },
      );

      const result = await service.render(
        NotificationType.DELIVERY_CREATED,
        NotificationChannel.IN_APP,
        'en',
        { a: 1 },
      );

      expect(result.title).toBe(
        `Notification: ${NotificationType.DELIVERY_CREATED}`,
      );
      expect(result.body).toBe(JSON.stringify({ a: 1 }));
    });
  });
});
