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

export enum MediaKafkaTopics {
  UPLOAD_CREATED = 'media.upload.created',
  UPLOAD_COMPLETED = 'media.upload.completed',
  UPLOAD_ABORTED = 'media.upload.aborted',
  SCAN_STARTED = 'media.scan.started',
  SCAN_COMPLETED = 'media.scan.completed',
  SCAN_FAILED = 'media.scan.failed',
  PROCESSING_STARTED = 'media.processing.started',
  PROCESSING_COMPLETED = 'media.processing.completed',
  PROCESSING_FAILED = 'media.processing.failed',
  MEDIA_READY = 'media.ready',
  DELETE_REQUESTED = 'media.delete.requested',
  MEDIA_DELETED = 'media.deleted',
  DELETE_FAILED = 'media.delete.failed',
}