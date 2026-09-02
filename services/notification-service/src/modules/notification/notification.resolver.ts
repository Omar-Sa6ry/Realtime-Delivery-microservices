import { Resolver, Query, Mutation, Args, ID, Int } from '@nestjs/graphql';
import { I18nService } from 'nestjs-i18n';
import {
  Auth,
  RedisRateLimit,
  RateLimiterAlgorithm,
  CurrentUser,
  Permission,
} from '@delivery/common';
import type { IUser } from '@delivery/common';
import { NotificationType } from '@delivery/common';
import { NotificationService } from './notification.service';
import { PreferenceService } from './preference/preference.service';
import {
  NotificationTypeObj,
  NotificationConnection,
  NotificationResponse,
  PaginatedNotificationResponse,
} from './dtos/notification.dto';
import { NotificationPreferenceResponse } from './dtos/preference.dto';
import { NotificationPreferenceInput } from './inputs/preference.input';
import { NotificationPreference } from '../../common/database/entities/notification-preference.entity';
import { IntResponse, BooleanResponse } from '@delivery/common';

const FIXED_WINDOW_RATE_LIMIT = { algorithm: RateLimiterAlgorithm.FIXED_WINDOW_COUNTER };

@Resolver(() => NotificationTypeObj)
export class NotificationResolver {
  constructor(
    private readonly notificationService: NotificationService,
    private readonly preferenceService: PreferenceService,
    private readonly i18n: I18nService,
  ) {}

  @Query(() => PaginatedNotificationResponse)
  @RedisRateLimit({ ...FIXED_WINDOW_RATE_LIMIT, limit: 100, windowMs: 60000 })
  @Auth([Permission.READ_NOTIFICATION])
  async myNotifications(
    @CurrentUser() user: IUser,
    @Args('page', { type: () => Int, nullable: true, defaultValue: 1 }) page: number,
    @Args('limit', { type: () => Int, nullable: true, defaultValue: 20 }) limit: number,
  ): Promise<PaginatedNotificationResponse> {
    const result = await this.notificationService.findAllForUser(user.id, page, limit);

    const notificationConnection: NotificationConnection = {
      items: result.items as NotificationTypeObj[],
      totalCount: result.totalCount,
    };

    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('notification.LIST_RETRIEVED'),
      data: notificationConnection,
    } as PaginatedNotificationResponse;
  }

  @Query(() => NotificationResponse)
  @RedisRateLimit({ ...FIXED_WINDOW_RATE_LIMIT, limit: 100, windowMs: 60000 })
  @Auth([Permission.READ_NOTIFICATION])
  async notification(
    @CurrentUser() user: IUser,
    @Args('id', { type: () => ID }) id: string,
  ): Promise<NotificationResponse> {
    const notification = await this.notificationService.findByIdForUser(id, user.id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('notification.GET_RETRIEVED'),
      data: notification || undefined,
    } as NotificationResponse;
  }

  @Query(() => IntResponse)
  @RedisRateLimit({ ...FIXED_WINDOW_RATE_LIMIT, limit: 100, windowMs: 60000 })
  @Auth([Permission.READ_NOTIFICATION])
  async unreadNotificationCount(@CurrentUser() user: IUser): Promise<IntResponse> {
    const count = await this.notificationService.unreadCountForUser(user.id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('notification.UNREAD_COUNT'),
      data: count,
    } as IntResponse;
  }

  @Mutation(() => NotificationResponse)
  @RedisRateLimit({ ...FIXED_WINDOW_RATE_LIMIT, limit: 60, windowMs: 60000 })
  @Auth([Permission.UPDATE_NOTIFICATION])
  async markNotificationAsRead(
    @CurrentUser() user: IUser,
    @Args('id', { type: () => ID }) id: string,
  ): Promise<NotificationResponse> {
    const notification = await this.notificationService.markAsRead(id, user.id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('notification.MARKED_READ'),
      data: notification,
    } as NotificationResponse;
  }

  @Mutation(() => BooleanResponse)
  @RedisRateLimit({ ...FIXED_WINDOW_RATE_LIMIT, limit: 60, windowMs: 60000 })
  @Auth([Permission.UPDATE_NOTIFICATION])
  async markAllNotificationsAsRead(@CurrentUser() user: IUser): Promise<BooleanResponse> {
    await this.notificationService.markAllAsRead(user.id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('notification.ALL_MARKED_READ'),
      data: true,
    };
  }

  @Mutation(() => BooleanResponse)
  @RedisRateLimit({ ...FIXED_WINDOW_RATE_LIMIT, limit: 50, windowMs: 60000 })
  @Auth([Permission.DELETE_NOTIFICATION])
  async deleteNotification(
    @CurrentUser() user: IUser,
    @Args('id', { type: () => ID }) id: string,
  ): Promise<BooleanResponse> {
    await this.notificationService.delete(id, user.id);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('notification.DELETED'),
      data: true,
    };
  }

  @Query(() => NotificationPreferenceResponse)
  @RedisRateLimit({ ...FIXED_WINDOW_RATE_LIMIT, limit: 100, windowMs: 60000 })
  @Auth([Permission.READ_NOTIFICATION])
  async myNotificationPreferences(
    @CurrentUser() user: IUser,
    @Args('type', { type: () => String, nullable: true }) type?: string,
  ): Promise<NotificationPreferenceResponse> {
    const items = await this.preferenceService.findForUser(user.id, type as NotificationType);
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('notification.PREFERENCES_RETRIEVED'),
      items,
    } as NotificationPreferenceResponse;
  }

  @Mutation(() => NotificationPreferenceResponse)
  @RedisRateLimit({ ...FIXED_WINDOW_RATE_LIMIT, limit: 60, windowMs: 60000 })
  @Auth([Permission.UPDATE_NOTIFICATION])
  async updateNotificationPreferences(
    @CurrentUser() user: IUser,
    @Args('preferences', { type: () => [NotificationPreferenceInput] }) preferences: NotificationPreferenceInput[],
  ): Promise<NotificationPreferenceResponse> {
    const saved: NotificationPreference[] = [];
    for (const preference of preferences) {
      const rows = await this.preferenceService.upsertPreferences(user.id, preference.type, preference.channels);
      saved.push(...rows);
    }
    return {
      success: true,
      statusCode: 200,
      message: await this.i18n.t('notification.PREFERENCES_UPDATED'),
      items: saved,
    } as NotificationPreferenceResponse;
  }
}

