export enum UserEvents {
  GET_USER_BY_ID = "user.get.by.id",
  GET_USER_BY_EMAIL = "user.get.by.email",
  USER_EXISTS = "user.exists",
  USER_UPDATED = "user.updated",
  USER_DATA_EXISTED = "user.dataExists",
  CREATE_USER_DATA = "user.createData",
  USER_ROLE_UPDATED = "user.role.updated",
  FIND_USERS_WITH_IDS = "user.findUsersWithIds",
  CHECK_IF_INSTRUCTOR = "user.checkIfInstructor",
}

// Kafka topics now live in `../kafka/kafka.topics`.
// Re-exported from here for backwards compatibility.
export { DeliveryKafkaTopics, PaymentKafkaTopics } from "../kafka/kafka.topics";

export enum NotificationNatsSubjects {
  NOTIFICATION_USER = 'notification.user', // + .{userId}
}

export enum RealtimeNatsSubjects {
  // Fan-out subjects (Realtime node → other nodes)
  LOCATION_DRIVER_UPDATED = 'realtime.location.driver.updated',
  DELIVERY_LOCATION_UPDATED = 'realtime.delivery.location.updated',
  DELIVERY_STATUS_UPDATED = 'realtime.delivery.status.updated',
  DRIVER_ASSIGNMENT_UPDATED = 'realtime.driver.assignment.updated',
  DRIVER_PRESENCE_UPDATED = 'realtime.driver.presence.updated',
  COMMAND_DRIVER = 'realtime.command.driver',
  COMMAND_DELIVERY = 'realtime.command.delivery',
}
