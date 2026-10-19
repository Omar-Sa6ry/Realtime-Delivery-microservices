import { Test, TestingModule } from '@nestjs/testing';
import { I18nService } from 'nestjs-i18n';
import { AppResolver } from './app.resolver';
import { createMockI18nService, createMockGraphQLContext } from '../../../test-utils';

describe('AppResolver', () => {
  let resolver: AppResolver;
  let i18n: ReturnType<typeof createMockI18nService>;

  beforeEach(async () => {
    i18n = createMockI18nService();

    const module: TestingModule = await Test.createTestingModule({
      providers: [
        AppResolver,
        { provide: I18nService, useValue: i18n },
      ],
    }).compile();

    resolver = module.get<AppResolver>(AppResolver);
  });

  it('should be defined', () => {
    expect(resolver).toBeDefined();
  });

  describe('deliveryServiceInfo', () => {
    it('should return service info', async () => {
      i18n.t.mockResolvedValue('Service info retrieved');

      const result = await resolver.deliveryServiceInfo(createMockGraphQLContext());

      expect(i18n.t).toHaveBeenCalledWith('delivery.serviceInfo', { lang: 'en' });
      expect(result).toEqual({
        success: true,
        statusCode: 200,
        message: 'Service info retrieved',
        data: {
          name: 'delivery-service',
          version: expect.any(String),
          status: 'ok',
        },
      });
    });

    it('should use context language', async () => {
      i18n.t.mockResolvedValue('Service info retrieved');

      await resolver.deliveryServiceInfo(createMockGraphQLContext({ language: 'ar' }));

      expect(i18n.t).toHaveBeenCalledWith('delivery.serviceInfo', { lang: 'ar' });
    });

    it('should use default language when context not provided', async () => {
      i18n.t.mockResolvedValue('Service info retrieved');

      await resolver.deliveryServiceInfo(undefined);

      expect(i18n.t).toHaveBeenCalledWith('delivery.serviceInfo', { lang: 'en' });
    });

    it('should use package version from env', async () => {
      process.env.npm_package_version = '1.2.3';
      i18n.t.mockResolvedValue('Service info retrieved');

      const result = await resolver.deliveryServiceInfo(createMockGraphQLContext());

      expect(result.data.version).toBe('1.2.3');
      delete process.env.npm_package_version;
    });

    it('should use default version when env not set', async () => {
      delete process.env.npm_package_version;
      i18n.t.mockResolvedValue('Service info retrieved');

      const result = await resolver.deliveryServiceInfo(createMockGraphQLContext());

      expect(result.data.version).toBe('0.0.1');
    });
  });
});