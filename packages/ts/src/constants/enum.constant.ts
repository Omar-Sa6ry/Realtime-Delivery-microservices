import { registerEnumType } from "@nestjs/graphql";

export enum Role {
  ADMIN = "admin",
  USER = "user",
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
