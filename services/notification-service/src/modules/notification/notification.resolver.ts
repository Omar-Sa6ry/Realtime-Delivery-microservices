import { Resolver, Query, Mutation, Args, ID, Int } from '@nestjs/graphql';
import { UseGuards } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository, IsNull } from 'typeorm';
import { Notification } from '../../common/database/entities/notification.entity';
import { Auth, CurrentUser } from '@delivery/common';
import { NotificationConnection, NotificationTypeObj } from './dtos/notification.dto';

@Resolver(() => NotificationTypeObj)
export class NotificationResolver {
  constructor(
    @InjectRepository(Notification)
    private notificationRepository: Repository<Notification>,
  ) {}

  @Query(() => NotificationConnection)
  @Auth(['READ_NOTIFICATION'])
  async myNotifications(
    @CurrentUser() user: any,
    @Args('page', { type: () => Int, nullable: true, defaultValue: 1 }) page: number,
    @Args('limit', { type: () => Int, nullable: true, defaultValue: 20 }) limit: number,
  ): Promise<NotificationConnection> {
    const [items, totalCount] = await this.notificationRepository.findAndCount({
      where: { userId: user.id },
      relations: { deliveries: true },
      order: { createdAt: 'DESC' },
      skip: (page - 1) * limit,
      take: limit,
    });

    return { items: items as any, totalCount };
  }

  @Query(() => NotificationTypeObj, { nullable: true })
  @Auth(['READ_NOTIFICATION'])
  async notification(
    @CurrentUser() user: any,
    @Args('id', { type: () => ID }) id: string,
  ) {
    return this.notificationRepository.findOne({
      where: { id, userId: user.id },
      relations: { deliveries: true },
    });
  }

  @Query(() => Int)
  @Auth(['READ_NOTIFICATION'])
  async unreadNotificationCount(@CurrentUser() user: any): Promise<number> {
    return this.notificationRepository.count({
      where: { userId: user.id, readAt: IsNull() },
    });
  }

  @Mutation(() => NotificationTypeObj)
  @Auth(['UPDATE_NOTIFICATION'])
  async markNotificationAsRead(
    @CurrentUser() user: any,
    @Args('id', { type: () => ID }) id: string,
  ) {
    const notification = await this.notificationRepository.findOne({ where: { id, userId: user.id } });
    if (!notification) throw new Error('Notification not found');

    notification.readAt = new Date();
    await this.notificationRepository.save(notification);
    return notification;
  }

  @Mutation(() => Boolean)
  @Auth(['UPDATE_NOTIFICATION'])
  async markAllNotificationsAsRead(@CurrentUser() user: any): Promise<boolean> {
    await this.notificationRepository.update({ userId: user.id, readAt: IsNull() }, { readAt: new Date() });
    return true;
  }

  @Mutation(() => Boolean)
  @Auth(['DELETE_NOTIFICATION'])
  async deleteNotification(
    @CurrentUser() user: any,
    @Args('id', { type: () => ID }) id: string,
  ): Promise<boolean> {
    const result = await this.notificationRepository.delete({ id, userId: user.id });
    return (result.affected ?? 0) > 0;
  }
}
