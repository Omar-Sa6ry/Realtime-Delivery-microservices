import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import * as Handlebars from 'handlebars';
import { NotificationTemplate } from '../../../common/database/entities/notification-template.entity';
import { NotificationType, NotificationChannel } from '@delivery/common';

@Injectable()
export class TemplateService implements OnModuleInit {
  private readonly logger = new Logger(TemplateService.name);
  // Cache parsed templates in memory
  private templatesCache = new Map<string, { title: Handlebars.TemplateDelegate; body: Handlebars.TemplateDelegate }>();

  constructor(
    @InjectRepository(NotificationTemplate)
    private templateRepository: Repository<NotificationTemplate>,
  ) {}

  async onModuleInit() {
    await this.loadTemplates();
  }

  async loadTemplates() {
    const templates = await this.templateRepository.find();
    for (const t of templates) {
      const key = `${t.type}:${t.channel}:${t.locale}`;
      try {
        this.templatesCache.set(key, {
          title: Handlebars.compile(t.titleTemplate),
          body: Handlebars.compile(t.bodyTemplate),
        });
      } catch (error) {
        this.logger.error(`Failed to compile template for ${key}: ${error.message}`);
      }
    }
    this.logger.log(`Loaded ${templates.length} templates`);
  }

  async render(type: NotificationType, channel: NotificationChannel, locale: string, data: any) {
    const key = `${type}:${channel}:${locale}`;
    let compiled = this.templatesCache.get(key);

    if (!compiled) {
      // Fallback to EN if specific locale not found
      if (locale !== 'en') {
        compiled = this.templatesCache.get(`${type}:${channel}:en`);
      }
    }

    if (!compiled) {
      this.logger.warn(`No template found for ${key}, falling back to default format.`);
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
      this.logger.error(`Error rendering template ${key}: ${error.message}`);
      return {
        title: `Notification: ${type}`,
        body: JSON.stringify(data),
      };
    }
  }
}
