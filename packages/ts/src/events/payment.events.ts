export interface PaymentCompletedPayload {
  paymentId: string;
  deliveryId: string;
  customerId: string;
  amount: number;
  currency: string;
  completedAt: string;
}

export interface PaymentFailedPayload {
  paymentId?: string;
  deliveryId: string;
  customerId: string;
  amount: number;
  currency: string;
  reason: string;
  failedAt: string;
}

export interface PaymentRefundedPayload {
  paymentId: string;
  deliveryId: string;
  amount: number;
  currency: string;
  reason: string;
  refundedAt: string;
}
