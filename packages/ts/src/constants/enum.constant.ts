import { registerEnumType } from "@nestjs/graphql";

export enum Role {
  ADMIN = "admin",
  USER = "user",
  CUSTOMER = "customer",
  DRIVER = "driver",
}
export const AllRoles: Role[] = Object.values(Role);

export enum Permission {
  // User
  UPDATE_USER = "update_user",
  DELETE_USER = "delete_user",
  EDIT_USER_ROLE = "edit_user_role",
  VIEW_USER = "view_user",

  // Auth
  RESET_PASSWORD = "RESET_PASSWORD",
  CHANGE_PASSWORD = "CHANGE_PASSWORD",
  FORGOT_PASSWORD = "FORGOT_PASSWORD",
  RECHARGE_WALLET = "RECHARGE_WALLET",
  LOGOUT = "LOGOUT",

  // Notification
  READ_NOTIFICATION = "READ_NOTIFICATION",
  UPDATE_NOTIFICATION = "UPDATE_NOTIFICATION",
  DELETE_NOTIFICATION = "DELETE_NOTIFICATION",
  MANAGE_NOTIFICATION_PREFERENCES = "MANAGE_NOTIFICATION_PREFERENCES",
}

export enum PaymentMethod {
  STRIPE = "STRIPE",
  PAYPAL = "PAYPAL",
  CASH = "CASH",
}

export enum PaymentStatus {
  PENDING = "PENDING",
  COMPLETED = "COMPLETED",
  FAILED = "FAILED",
  REFUNDED = "REFUNDED",
}

export enum HeaderKeys {
  X_USER_ID = "x-user-id",
  X_USER_ROLE = "x-user-role",
  X_USER_SESSION = "x-user-session",
  X_CORRELATION_ID = "x-correlation-id",
}

export enum NotificationType {
  DELIVERY_CREATED = "DELIVERY_CREATED",
  DRIVER_ASSIGNED = "DRIVER_ASSIGNED",
  DRIVER_ACCEPTED = "DRIVER_ACCEPTED",
  DELIVERY_PICKED_UP = "DELIVERY_PICKED_UP",
  DELIVERY_IN_TRANSIT = "DELIVERY_IN_TRANSIT",
  DELIVERY_COMPLETED = "DELIVERY_COMPLETED",
  DELIVERY_CANCELLED = "DELIVERY_CANCELLED",
  PAYMENT_COMPLETED = "PAYMENT_COMPLETED",
  PAYMENT_FAILED = "PAYMENT_FAILED",
  PAYMENT_REFUNDED = "PAYMENT_REFUNDED",
}

export enum NotificationStatus {
  CREATED = "CREATED",
  QUEUED = "QUEUED",
  PROCESSING = "PROCESSING",
  SENT = "SENT",
  DELIVERED = "DELIVERED",
  FAILED = "FAILED",
  CANCELLED = "CANCELLED",
  EXPIRED = "EXPIRED",
}

export enum DeliveryChannelStatus {
  PENDING = "PENDING",
  QUEUED = "QUEUED",
  PROCESSING = "PROCESSING",
  RETRYING = "RETRYING",
  SENT = "SENT",
  DELIVERED = "DELIVERED",
  FAILED = "FAILED",
}

export enum NotificationChannel {
  EMAIL = "EMAIL",
  SMS = "SMS",
  PUSH = "PUSH",
  IN_APP = "IN_APP",
  REALTIME = "REALTIME",
}

export enum NotificationPriority {
  LOW = "LOW",
  NORMAL = "NORMAL",
  HIGH = "HIGH",
  CRITICAL = "CRITICAL",
}

registerEnumType(Role, {
  name: "Role",
  description: "User roles in the system",
});

registerEnumType(PaymentMethod, {
  name: "PaymentMethod",
  description: "Supported payment methods",
});

registerEnumType(PaymentStatus, {
  name: "PaymentStatus",
  description: "Status of payment transactions",
});

registerEnumType(NotificationType, {
  name: "NotificationType",
  description: "Types of notifications",
});

registerEnumType(NotificationStatus, {
  name: "NotificationStatus",
  description: "Status of notification processing",
});

registerEnumType(DeliveryChannelStatus, {
  name: "DeliveryChannelStatus",
  description: "Status of specific notification delivery channel",
});

registerEnumType(NotificationChannel, {
  name: "NotificationChannel",
  description: "Channels for notification delivery",
});

registerEnumType(NotificationPriority, {
  name: "NotificationPriority",
  description: "Priority of notification delivery",
});
