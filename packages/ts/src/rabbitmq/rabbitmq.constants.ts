export enum RabbitMQExchanges {
  ORDERS = 'delivery.orders.topic',
  NOTIFICATIONS = 'delivery.notifications.fanout',
  PAYMENTS = 'delivery.payments.direct',
  DISPATCH = 'delivery.dispatch.direct',
  MEDIA = 'delivery.media.topic',
  REALTIME = 'delivery.realtime.fanout',
  USERS = 'delivery.users.topic',
  ANALYTICS = 'delivery.analytics.fanout',
  DRIVERS = 'delivery.drivers.topic',
  DLX = 'delivery.dlx.fanout',
  DLX_DIRECT = 'delivery.dlx.direct',
  UNROUTABLE = 'delivery.unroutable.fanout',
}

export enum RabbitMQExchangeType {
  TOPIC = 'topic',
  FANOUT = 'fanout',
  DIRECT = 'direct',
}

export const RABBITMQ_EXCHANGE_TYPES: Record<RabbitMQExchanges, RabbitMQExchangeType> = {
  [RabbitMQExchanges.ORDERS]: RabbitMQExchangeType.TOPIC,
  [RabbitMQExchanges.NOTIFICATIONS]: RabbitMQExchangeType.FANOUT,
  [RabbitMQExchanges.PAYMENTS]: RabbitMQExchangeType.DIRECT,
  [RabbitMQExchanges.DISPATCH]: RabbitMQExchangeType.DIRECT,
  [RabbitMQExchanges.MEDIA]: RabbitMQExchangeType.TOPIC,
  [RabbitMQExchanges.REALTIME]: RabbitMQExchangeType.FANOUT,
  [RabbitMQExchanges.USERS]: RabbitMQExchangeType.TOPIC,
  [RabbitMQExchanges.ANALYTICS]: RabbitMQExchangeType.FANOUT,
  [RabbitMQExchanges.DRIVERS]: RabbitMQExchangeType.TOPIC,
  [RabbitMQExchanges.DLX]: RabbitMQExchangeType.FANOUT,
  [RabbitMQExchanges.DLX_DIRECT]: RabbitMQExchangeType.DIRECT,
  [RabbitMQExchanges.UNROUTABLE]: RabbitMQExchangeType.FANOUT,
};

export enum OrdersRoutingKeys {
  CREATED = 'order.created',
  STATUS_ALL = 'order.#',
  CANCELLED = 'order.cancelled',
  COMPLETED = 'order.completed',
  PICKED_UP = 'order.picked_up',
  IN_TRANSIT = 'order.in_transit',
  DRIVER_ASSIGNED = 'order.driver.assigned',
}

export enum PaymentsRoutingKeys {
  AUTHORIZED = 'payment.authorized',
  FAILED = 'payment.failed',
  REFUNDED = 'payment.refunded',
  COMPLETED = 'payment.completed',
}

export enum DispatchRoutingKeys {
  REQUEST = 'dispatch.request',
  RESPONSE = 'dispatch.response',
}

export enum MediaRoutingKeys {
  UPLOADED = 'media.uploaded',
  PROCESSED = 'media.processed',
  ALL = 'media.#',
}

export enum UsersRoutingKeys {
  CREATED = 'user.created',
  UPDATED = 'user.updated',
  ALL = 'user.#',
}

export enum DriversRoutingKeys {
  EVENTS_ALL = 'driver.#',
  CREATED = 'driver.created',
  UPDATED = 'driver.updated',
  ASSIGNMENT_OFFERED = 'driver.assignment.offered',
  ASSIGNMENT_ACCEPTED = 'driver.assignment.accepted',
}

export enum DlqRoutingKeys {
  ORDERS = 'orders.dlq',
  PAYMENTS = 'payments.dlq',
  NOTIFICATIONS = 'notifications.dlq',
  DRIVERS = 'drivers.dlq',
}

export enum RabbitMQQueues {
  ORDERS_CREATED = 'orders.created.queue',
  ORDERS_STATUS = 'orders.status.queue',
  NOTIFICATIONS_EMAIL = 'notifications.email.queue',
  NOTIFICATIONS_SMS = 'notifications.sms.queue',
  NOTIFICATIONS_PUSH = 'notifications.push.queue',
  PAYMENTS_AUTHORIZED = 'payments.authorized.queue',
  PAYMENTS_FAILED = 'payments.failed.queue',
  PAYMENTS_REFUNDED = 'payments.refunded.queue',
  DISPATCH_REQUESTS = 'dispatch.requests.queue',
  DISPATCH_RESPONSES = 'dispatch.responses.queue',
  MEDIA_UPLOADED = 'media.uploaded.queue',
  MEDIA_PROCESSED = 'media.processed.queue',
  REALTIME_BROADCAST = 'realtime.broadcast.queue',
  SEARCH_INDEX = 'search.index.queue',
  DELIVERY_DLQ = 'delivery.dlq.queue',
  USERS_CREATED = 'users.created.queue',
  USERS_UPDATED = 'users.updated.queue',
  ANALYTICS_EVENTS = 'analytics.events.queue',
  DRIVERS_EVENTS = 'drivers.events.queue',
  DLQ_ORDERS = 'orders.dlq.queue',
  DLQ_PAYMENTS = 'payments.dlq.queue',
  DLQ_NOTIFICATIONS = 'notifications.dlq.queue',
  DLQ_DRIVERS = 'drivers.dlq.queue',
  UNROUTABLE_MESSAGES = 'unroutable.messages.queue',
}

export const RABBITMQ_QUEUE_DLQ: Record<string, { exchange: string; routingKey: string }> = {
  [RabbitMQQueues.ORDERS_CREATED]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.ORDERS,
  },
  [RabbitMQQueues.ORDERS_STATUS]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.ORDERS,
  },
  [RabbitMQQueues.PAYMENTS_AUTHORIZED]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.PAYMENTS,
  },
  [RabbitMQQueues.PAYMENTS_FAILED]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.PAYMENTS,
  },
  [RabbitMQQueues.PAYMENTS_REFUNDED]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.PAYMENTS,
  },
  [RabbitMQQueues.NOTIFICATIONS_EMAIL]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.NOTIFICATIONS,
  },
  [RabbitMQQueues.NOTIFICATIONS_SMS]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.NOTIFICATIONS,
  },
  [RabbitMQQueues.NOTIFICATIONS_PUSH]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.NOTIFICATIONS,
  },
  [RabbitMQQueues.DISPATCH_REQUESTS]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.DRIVERS,
  },
  [RabbitMQQueues.DISPATCH_RESPONSES]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.DRIVERS,
  },
  [RabbitMQQueues.DRIVERS_EVENTS]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.DRIVERS,
  },
  [RabbitMQQueues.MEDIA_UPLOADED]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.NOTIFICATIONS,
  },
  [RabbitMQQueues.MEDIA_PROCESSED]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.NOTIFICATIONS,
  },
  [RabbitMQQueues.REALTIME_BROADCAST]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.ORDERS,
  },
  [RabbitMQQueues.SEARCH_INDEX]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.ORDERS,
  },
  [RabbitMQQueues.USERS_CREATED]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.NOTIFICATIONS,
  },
  [RabbitMQQueues.USERS_UPDATED]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.NOTIFICATIONS,
  },
  [RabbitMQQueues.ANALYTICS_EVENTS]: {
    exchange: RabbitMQExchanges.DLX_DIRECT,
    routingKey: DlqRoutingKeys.NOTIFICATIONS,
  },
};

export function buildQueueArguments(queue: string): Record<string, unknown> {
  const isDlq = queue.endsWith('.dlq.queue') || queue === RabbitMQQueues.DELIVERY_DLQ;
  if (isDlq) {
    return {
      'x-queue-type': 'quorum',
      'x-message-ttl': 604800000, // 7 days retention for DLQ
    };
  }

  if (queue === RabbitMQQueues.ANALYTICS_EVENTS || queue === RabbitMQQueues.REALTIME_BROADCAST) {
    return {
      'x-queue-type': 'stream',
      'x-max-age': '7D',
      'x-stream-max-segment-size-bytes': 50000000,
    };
  }

  const dlq = RABBITMQ_QUEUE_DLQ[queue];
  return {
    'x-queue-type': 'quorum',
    ...(dlq
      ? {
          'x-dead-letter-exchange': dlq.exchange,
          'x-dead-letter-routing-key': dlq.routingKey,
        }
      : {
          'x-dead-letter-exchange': RabbitMQExchanges.DLX,
        }),
  };
}

export enum RabbitMQHeaders {
  RETRY_COUNT = 'x-retry-count',
  TRACE_ID = 'x-trace-id',
  DLQ_REASON = 'x-dlq-reason',
}

export const RABBITMQ_PREFETCH: Record<string, number> = {
  [RabbitMQQueues.NOTIFICATIONS_EMAIL]: 10,
  [RabbitMQQueues.NOTIFICATIONS_SMS]: 20,
  [RabbitMQQueues.NOTIFICATIONS_PUSH]: 50,
  DEFAULT: 10,
};

export function resolvePrefetch(queue: string, fallback = 10): number {
  return RABBITMQ_PREFETCH[queue] ?? RABBITMQ_PREFETCH.DEFAULT ?? fallback;
}
