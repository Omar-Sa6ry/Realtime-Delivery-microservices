import { GrpcController } from './grpc.controller';
import { NotificationService } from '../notification/notification.service';
import { NotificationType } from '@delivery-micro/shard';

describe('GrpcController', () => {
  let controller: GrpcController;
  let notificationService: { createAndDispatch: jest.Mock };

  beforeEach(() => {
    notificationService = { createAndDispatch: jest.fn() };
    controller = new GrpcController(
      notificationService as unknown as NotificationService,
    );
  });

  it('parses the JSON data payload and returns the notification id', async () => {
    notificationService.createAndDispatch.mockResolvedValue('n-123');

    const result = await controller.sendNotification({
      userId: 'u-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 'Hello',
      body: 'World',
      data: JSON.stringify({ orderId: 'o-1' }),
      priority: 'HIGH',
    });

    expect(notificationService.createAndDispatch).toHaveBeenCalledWith({
      userId: 'u-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 'Hello',
      body: 'World',
      data: { orderId: 'o-1' },
      priority: 'HIGH',
    });
    expect(result).toEqual({ notificationId: 'n-123', success: true });
  });

  it('tolerates malformed JSON without crashing and uses empty data', async () => {
    notificationService.createAndDispatch.mockResolvedValue('n-1');

    const result = await controller.sendNotification({
      userId: 'u-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 't',
      body: 'b',
      data: '{not-json',
      priority: 'NORMAL',
    });

    expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
      expect.objectContaining({ data: {} }),
    );
    expect(result.success).toBe(true);
  });

  it('handles a missing data field', async () => {
    notificationService.createAndDispatch.mockResolvedValue('n-2');

    await controller.sendNotification({
      userId: 'u-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 't',
      body: 'b',
      data: '',
      priority: 'LOW',
    });

    expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
      expect.objectContaining({ data: {} }),
    );
  });

  it('returns success: false when notification creation fails', async () => {
    notificationService.createAndDispatch.mockResolvedValue(null);

    const result = await controller.sendNotification({
      userId: 'u-1',
      type: NotificationType.DELIVERY_CREATED,
      title: 't',
      body: 'b',
      data: '',
      priority: 'NORMAL',
    });

    expect(result).toEqual({ notificationId: null, success: false });
  });
});
