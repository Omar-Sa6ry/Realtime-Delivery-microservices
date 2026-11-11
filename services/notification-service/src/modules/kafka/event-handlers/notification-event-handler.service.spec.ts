import { NotificationEventHandler } from './notification-event-handler.service';
import { EventHandlerFactory } from './event-handler.factory';
import { NotificationService } from '../../notification/notification.service';
import {
  DeliveryKafkaTopics,
  MediaKafkaTopics,
  NotificationChannel,
  NotificationPriority,
  NotificationType,
  PaymentKafkaTopics,
  UserKafkaTopics,
} from '@delivery-micro/shard';

describe('NotificationEventHandler', () => {
  let handler: NotificationEventHandler;
  let notificationService: { createAndDispatch: jest.Mock };
  let factory: EventHandlerFactory;

  beforeEach(() => {
    notificationService = {
      createAndDispatch: jest.fn().mockResolvedValue('n-1'),
    };
    factory = new EventHandlerFactory();
    handler = new NotificationEventHandler(
      notificationService as unknown as NotificationService,
      factory,
    );
  });

  describe('onModuleInit', () => {
    it('registers a handler for every mapped event type', () => {
      handler.onModuleInit();

      expect(factory.getHandler(DeliveryKafkaTopics.DELIVERY_CREATED)).toBe(
        handler,
      );
      expect(factory.getHandler(PaymentKafkaTopics.PAYMENT_FAILED)).toBe(
        handler,
      );
      expect(factory.getHandler(MediaKafkaTopics.MEDIA_READY)).toBe(handler);
      expect(factory.getHandler(UserKafkaTopics.PASSWORD_RESET_REQUESTED)).toBe(
        handler,
      );
    });
  });

  describe('handle', () => {
    it('ignores payloads without eventType', async () => {
      await handler.handle({ eventId: 'e-1', userId: 'u-1' });

      expect(notificationService.createAndDispatch).not.toHaveBeenCalled();
    });

    it('ignores unmapped event types', async () => {
      await handler.handle({
        eventId: 'e-1',
        eventType: 'totally.unknown',
        userId: 'u-1',
      });

      expect(notificationService.createAndDispatch).not.toHaveBeenCalled();
    });

    it('skips events without any resolvable user id', async () => {
      await handler.handle({
        eventId: 'e-1',
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
      });

      expect(notificationService.createAndDispatch).not.toHaveBeenCalled();
    });

    it.each([
      ['top-level userId', { userId: 'u-top' }, 'u-top'],
      ['top-level user_id', { user_id: 'u-snake' }, 'u-snake'],
      ['payload.userId', { payload: { userId: 'u-payload' } }, 'u-payload'],
      ['payload.user_id', { payload: { user_id: 'u-p-snake' } }, 'u-p-snake'],
      ['payload.customerId', { payload: { customerId: 'c-1' } }, 'c-1'],
      ['payload.customer_id', { payload: { customer_id: 'c-2' } }, 'c-2'],
      ['payload.driverId', { payload: { driverId: 'd-1' } }, 'd-1'],
      ['payload.driver_id', { payload: { driver_id: 'd-2' } }, 'd-2'],
      ['data.userId', { data: { userId: 'u-data' } }, 'u-data'],
    ])('extracts userId from %s', async (_label, extra, expected) => {
      await handler.handle({
        eventId: 'e-1',
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
        ...extra,
      });

      expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
        expect.objectContaining({ userId: expected }),
      );
    });

    it('maps delivery.created events to the DELIVERY_CREATED notification type', async () => {
      await handler.handle({
        eventId: 'e-1',
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
        userId: 'u-1',
        payload: { deliveryId: 'del-1' },
      });

      expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
        expect.objectContaining({
          userId: 'u-1',
          type: NotificationType.DELIVERY_CREATED,
          data: expect.objectContaining({
            deliveryId: 'del-1',
            eventId: 'e-1',
          }),
        }),
      );
    });

    it('marks driver.assigned as HIGH priority', async () => {
      await handler.handle({
        eventId: 'e-1',
        eventType: DeliveryKafkaTopics.DRIVER_ASSIGNED,
        userId: 'u-1',
      });

      expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
        expect.objectContaining({
          type: NotificationType.DRIVER_ASSIGNED,
          priority: NotificationPriority.HIGH,
        }),
      );
    });

    it('forces an IN_APP channel for password reset security events', async () => {
      await handler.handle({
        eventId: 'e-1',
        eventType: UserKafkaTopics.PASSWORD_RESET_REQUESTED,
        userId: 'u-1',
      });

      expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
        expect.objectContaining({
          type: NotificationType.PASSWORD_RESET_REQUESTED,
          priority: NotificationPriority.HIGH,
          requiredChannels: [NotificationChannel.IN_APP],
        }),
      );
    });

    it.each([
      [
        PaymentKafkaTopics.PAYMENT_COMPLETED,
        NotificationType.PAYMENT_COMPLETED,
      ],
      [PaymentKafkaTopics.PAYMENT_FAILED, NotificationType.PAYMENT_FAILED],
      [PaymentKafkaTopics.PAYMENT_REFUNDED, NotificationType.PAYMENT_REFUNDED],
      [DeliveryKafkaTopics.DRIVER_ACCEPTED, NotificationType.DRIVER_ACCEPTED],
      [
        DeliveryKafkaTopics.DELIVERY_PICKED_UP,
        NotificationType.DELIVERY_PICKED_UP,
      ],
      [
        DeliveryKafkaTopics.DELIVERY_IN_TRANSIT,
        NotificationType.DELIVERY_IN_TRANSIT,
      ],
      [
        DeliveryKafkaTopics.DELIVERY_COMPLETED,
        NotificationType.DELIVERY_COMPLETED,
      ],
      [
        DeliveryKafkaTopics.DELIVERY_CANCELLED,
        NotificationType.DELIVERY_CANCELLED,
      ],
      [
        MediaKafkaTopics.UPLOAD_COMPLETED,
        NotificationType.MEDIA_UPLOAD_COMPLETED,
      ],
      [MediaKafkaTopics.UPLOAD_ABORTED, NotificationType.MEDIA_UPLOAD_FAILED],
      [MediaKafkaTopics.SCAN_FAILED, NotificationType.MEDIA_SCAN_FAILED],
      [
        MediaKafkaTopics.PROCESSING_FAILED,
        NotificationType.MEDIA_PROCESSING_FAILED,
      ],
      [MediaKafkaTopics.MEDIA_READY, NotificationType.MEDIA_READY],
      [MediaKafkaTopics.MEDIA_DELETED, NotificationType.MEDIA_DELETED],
      [MediaKafkaTopics.DELETE_FAILED, NotificationType.MEDIA_DELETE_FAILED],
      [UserKafkaTopics.USER_CREATED, NotificationType.USER_REGISTERED],
    ])('maps %s to %s', async (eventType, expectedType) => {
      await handler.handle({
        eventId: 'e-1',
        eventType,
        userId: 'u-1',
      });

      expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
        expect.objectContaining({ type: expectedType }),
      );
    });

    it('falls back to payload.id as the event id when eventId is missing', async () => {
      await handler.handle({
        id: 'legacy-id',
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
        userId: 'u-1',
      });

      expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
        expect.objectContaining({
          data: expect.objectContaining({ eventId: 'legacy-id' }),
        }),
      );
    });

    it('merges both payload and data into the notification data', async () => {
      await handler.handle({
        eventId: 'e-1',
        eventType: DeliveryKafkaTopics.DELIVERY_CREATED,
        userId: 'u-1',
        payload: { a: 1 },
        data: { b: 2 },
      });

      expect(notificationService.createAndDispatch).toHaveBeenCalledWith(
        expect.objectContaining({
          data: expect.objectContaining({ a: 1, b: 2, eventId: 'e-1' }),
        }),
      );
    });
  });
});
