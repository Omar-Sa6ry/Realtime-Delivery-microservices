import { Controller } from '@nestjs/common';
import { GrpcMethod } from '@nestjs/microservices';
import { NotificationService } from '../notification/notification.service';
import { NotificationPriority, NotificationType } from '@delivery/common';

export interface SendNotificationRequest {
  userId: string;
  type: string;
  title: string;
  body: string;
  data: string; // JSON
  priority: string;
}

@Controller()
export class GrpcController {
  constructor(private readonly notificationService: NotificationService) {}

  @GrpcMethod('NotificationService', 'SendNotification')
  async sendNotification(data: SendNotificationRequest) {
    let parsedData = {};
    if (data.data) {
      try {
        parsedData = JSON.parse(data.data);
      } catch (e) {
        // ignore JSON parse error
      }
    }

    const notificationId = await this.notificationService.createAndDispatch({
      userId: data.userId,
      type: data.type as NotificationType,
      title: data.title,
      body: data.body,
      data: parsedData,
      priority: data.priority as NotificationPriority,
    });

    return {
      notificationId,
      success: !!notificationId,
    };
  }
}
