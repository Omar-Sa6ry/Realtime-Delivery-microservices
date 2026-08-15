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

export enum DeliveryKafkaTopics {
  DELIVERY_CREATED = 'delivery.created',
  DRIVER_ASSIGNED = 'delivery.driver.assigned',
  DRIVER_ACCEPTED = 'delivery.driver.accepted',
  DELIVERY_PICKED_UP = 'delivery.picked_up',
  DELIVERY_IN_TRANSIT = 'delivery.in_transit',
  DELIVERY_COMPLETED = 'delivery.completed',
  DELIVERY_CANCELLED = 'delivery.cancelled',
}

export enum PaymentKafkaTopics {
  PAYMENT_COMPLETED = 'payment.completed',
  PAYMENT_FAILED = 'payment.failed',
  PAYMENT_REFUNDED = 'payment.refunded',
}

export enum NotificationNatsSubjects {
  NOTIFICATION_USER = 'notification.user', // + .{userId}
}
