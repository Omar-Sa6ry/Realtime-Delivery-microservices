import {
  UserEvents,
  NotificationNatsSubjects,
  RealtimeNatsSubjects,
  DeliveryKafkaTopics,
  PaymentKafkaTopics,
} from './events';
import * as kafkaTopics from '../kafka/kafka.topics';

describe('nats events', () => {
  describe('UserEvents', () => {
    it('exposes every user subject', () => {
      expect(Object.keys(UserEvents)).toHaveLength(9);
      expect(UserEvents.GET_USER_BY_ID).toBe('user.get.by.id');
      expect(UserEvents.GET_USER_BY_EMAIL).toBe('user.get.by.email');
      expect(UserEvents.USER_EXISTS).toBe('user.exists');
      expect(UserEvents.USER_UPDATED).toBe('user.updated');
      expect(UserEvents.USER_DATA_EXISTED).toBe('user.dataExists');
      expect(UserEvents.CREATE_USER_DATA).toBe('user.createData');
      expect(UserEvents.USER_ROLE_UPDATED).toBe('user.role.updated');
      expect(UserEvents.FIND_USERS_WITH_IDS).toBe('user.findUsersWithIds');
      expect(UserEvents.CHECK_IF_INSTRUCTOR).toBe('user.checkIfInstructor');
    });
  });

  describe('NotificationNatsSubjects', () => {
    it('exposes the notification subject', () => {
      expect(NotificationNatsSubjects.NOTIFICATION_USER).toBe('notification.user');
    });
  });

  describe('RealtimeNatsSubjects', () => {
    it('exposes every realtime fan-out subject', () => {
      expect(Object.keys(RealtimeNatsSubjects)).toHaveLength(13);
      expect(RealtimeNatsSubjects.LOCATION_DRIVER_UPDATED).toBe(
        'realtime.location.driver.updated',
      );
      expect(RealtimeNatsSubjects.DELIVERY_LOCATION_UPDATED).toBe(
        'realtime.delivery.location.updated',
      );
      expect(RealtimeNatsSubjects.DELIVERY_STATUS_UPDATED).toBe(
        'realtime.delivery.status.updated',
      );
      expect(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_UPDATED).toBe(
        'realtime.driver.assignment.updated',
      );
      expect(RealtimeNatsSubjects.DRIVER_ASSIGNMENT_OFFERED).toBe(
        'realtime.driver.assignment.offered',
      );
      expect(RealtimeNatsSubjects.DRIVER_PRESENCE_UPDATED).toBe(
        'realtime.driver.presence.updated',
      );
      expect(RealtimeNatsSubjects.COMMAND_DRIVER).toBe('realtime.command.driver');
      expect(RealtimeNatsSubjects.COMMAND_DELIVERY).toBe('realtime.command.delivery');
      expect(RealtimeNatsSubjects.MEDIA_UPLOAD_PROGRESS).toBe('realtime.media.upload.progress');
      expect(RealtimeNatsSubjects.MEDIA_PROCESSING_PROGRESS).toBe(
        'realtime.media.processing.progress',
      );
      expect(RealtimeNatsSubjects.MEDIA_READY).toBe('realtime.media.ready');
      expect(RealtimeNatsSubjects.MEDIA_DELETED).toBe('realtime.media.deleted');
      expect(RealtimeNatsSubjects.MEDIA_FAILED).toBe('realtime.media.failed');
    });
  });

  describe('re-exports', () => {
    it('re-exports the kafka topic enums', () => {
      expect(DeliveryKafkaTopics).toBe(kafkaTopics.DeliveryKafkaTopics);
      expect(PaymentKafkaTopics).toBe(kafkaTopics.PaymentKafkaTopics);
      expect(DeliveryKafkaTopics.DELIVERY_CREATED).toBe('delivery.created');
      expect(PaymentKafkaTopics.PAYMENT_COMPLETED).toBe('payment.completed');
    });
  });
});
