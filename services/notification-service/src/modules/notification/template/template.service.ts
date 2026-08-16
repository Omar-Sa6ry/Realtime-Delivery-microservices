import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import * as Handlebars from 'handlebars';
import { RedisService } from '@bts-soft/cache';
import { NotificationTemplate } from '../../../common/database/entities/notification-template.entity';
import { NotificationType, NotificationChannel } from '@delivery/common';

interface CompiledTemplate {
  title: Handlebars.TemplateDelegate;
  body: Handlebars.TemplateDelegate;
}

interface RawTemplate {
  type: NotificationType;
  channel: NotificationChannel;
  locale: string;
  titleTemplate: string;
  bodyTemplate: string;
}

const TEMPLATE_CACHE_TTL_SECONDS = 3600;

const DEFAULT_TEMPLATES: RawTemplate[] = [
  {
    type: NotificationType.PASSWORD_RESET_REQUESTED,
    channel: NotificationChannel.IN_APP,
    locale: 'en',
    titleTemplate: 'Password Reset Requested',
    bodyTemplate:
      'A password reset was requested for your account. {{firstName}}, check your email for the reset code. It expires in 15 minutes.',
  },
  {
    type: NotificationType.PASSWORD_RESET_REQUESTED,
    channel: NotificationChannel.IN_APP,
    locale: 'ar',
    titleTemplate: 'تم طلب إعادة تعيين كلمة المرور',
    bodyTemplate:
      'تم طلب إعادة تعيين كلمة المرور لحسابك. يا {{firstName}}، راجع بريدك الإلكتروني لمعرفة رمز إعادة التعيين. صلاحية الرمز 15 دقيقة.',
  },
];

@Injectable()
export class TemplateService implements OnModuleInit {
  private readonly logger = new Logger(TemplateService.name);
  private templatesCache = new Map<string, CompiledTemplate>();

  constructor(
    @InjectRepository(NotificationTemplate)
    private templateRepository: Repository<NotificationTemplate>,
    private readonly redisService: RedisService,
  ) {}

  async onModuleInit() {
    await this.seedDefaultTemplates();
    await this.loadTemplates();
  }

  private async seedDefaultTemplates() {
    for (const raw of DEFAULT_TEMPLATES) {
      const existing = await this.templateRepository.findOne({
        where: { type: raw.type, channel: raw.channel, locale: raw.locale },
      });
      if (existing) continue;

      const entity = this.templateRepository.create({
        type: raw.type,
        channel: raw.channel,
        locale: raw.locale,
        titleTemplate: raw.titleTemplate,
        bodyTemplate: raw.bodyTemplate,
      });
      await this.templateRepository.save(entity);
    }
    this.logger.log(`Seeded ${DEFAULT_TEMPLATES.length} default templates`);
  }

  async loadTemplates() {
    const templates = await this.templateRepository.find();
    for (const t of templates) {
      const key = `${t.type}:${t.channel}:${t.locale}`;
      const compiled = this.compile(t.titleTemplate, t.bodyTemplate);
      if (compiled) {
        this.templatesCache.set(key, compiled);
      }
    }
    this.logger.log(`Loaded ${templates.length} templates`);
  }

  async render(type: NotificationType, channel: NotificationChannel, locale: string, data: Record<string, unknown>) {
    let compiled = await this.resolveTemplate(type, channel, locale);

    // Fallback to EN if specific locale not found
    if (!compiled && locale !== 'en') {
      compiled = await this.resolveTemplate(type, channel, 'en');
    }

    if (!compiled) {
      this.logger.warn(`No template found for ${type}:${channel}:${locale}, falling back to default format.`);
      return {
        title: `Notification: ${type}`,
        body: JSON.stringify(data),
      };
    }

    try {
      return {
        title: compiled.title(data),
        body: compiled.body(data),
      };
    } catch (error) {
      this.logger.error(`Error rendering template ${type}:${channel}:${locale}: ${(error as Error).message}`);
      return {
        title: `Notification: ${type}`,
        body: JSON.stringify(data),
      };
    }
  }

  private async resolveTemplate(
    type: NotificationType,
    channel: NotificationChannel,
    locale: string,
  ): Promise<CompiledTemplate | null> {
    const cacheKey = `${type}:${channel}:${locale}`;

    const inMemory = this.templatesCache.get(cacheKey);
    if (inMemory) {
      return inMemory;
    }

    const redisKey = `template:${cacheKey}`;
    const cachedRow = await this.redisService.get<RawTemplate>(redisKey);
    if (cachedRow) {
      return this.toCompiled(cacheKey, cachedRow);
    }

    const entity = await this.templateRepository.findOne({
      where: { type, channel, locale },
    });

    if (!entity) {
      return null;
    }

    const raw: RawTemplate = {
      type: entity.type,
      channel: entity.channel,
      locale: entity.locale,
      titleTemplate: entity.titleTemplate,
      bodyTemplate: entity.bodyTemplate,
    };

    await this.redisService.set(redisKey, raw, TEMPLATE_CACHE_TTL_SECONDS);
    return this.toCompiled(cacheKey, raw);
  }

  private toCompiled(cacheKey: string, raw: RawTemplate): CompiledTemplate | null {
    const compiled = this.compile(raw.titleTemplate, raw.bodyTemplate);
    if (compiled) {
      this.templatesCache.set(cacheKey, compiled);
    }
    return compiled;
  }

  private compile(titleTemplate: string, bodyTemplate: string): CompiledTemplate | null {
    try {
      return {
        title: Handlebars.compile(titleTemplate),
        body: Handlebars.compile(bodyTemplate),
      };
    } catch (error) {
      this.logger.error(`Failed to compile template: ${(error as Error).message}`);
      return null;
    }
  }
}