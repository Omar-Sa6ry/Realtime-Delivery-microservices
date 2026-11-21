import { ApolloServer } from '@apollo/server';
import { startStandaloneServer } from '@apollo/server/standalone';
import { buildSubgraphSchema } from '@apollo/subgraph';
import { parse } from 'graphql';
import type { GraphQLResolveInfo } from 'graphql';
import { SUBGRAPH_SDL } from './sdl';

/** A single recorded subgraph request (headers forwarded by the gateway). */
export interface CapturedRequest {
  subgraph: string;
  /** Operation text the gateway sent (for debugging). */
  query?: string;
  variables?: Record<string, unknown>;
  headers: Record<string, string | undefined>;
  /** The root field(s) that were resolved during this request. */
  fields: string[];
}

export interface MockSubgraphContext {
  /** All requests captured so far (newest last). */
  readonly captured: CapturedRequest[];
  /** Requests captured for one specific subgraph. */
  forSubgraph(name: string): CapturedRequest[];
  /** Most recent request captured for a subgraph (or undefined). */
  last(name: string): CapturedRequest | undefined;
  /** Drop all captured requests. */
  clear(): void;
  /** Make a specific root field throw a gateway-visible error once. */
  injectError(
    subgraph: string,
    field: string,
    error: { message: string; extensions?: Record<string, unknown> },
  ): void;
  /** Clear any injected errors. */
  clearErrors(): void;
  /** URLs the gateway should be pointed at, keyed by subgraph name. */
  readonly urls: Record<string, string>;
  /** Stop all mock servers. */
  stop(): Promise<void>;
}

/**
 * Builds the resolver map for a subgraph. Every root field records the request
 * (so tests can assert header propagation + which subgraph served the call)
 * and returns canned, schema-valid data. Special sentinel values in arguments
 * trigger injected errors so the gateway's `formatError` can be verified.
 */
function buildResolvers(
  name: string,
  ctx: {
    captured: CapturedRequest[];
    errors: Map<
      string,
      { message: string; extensions?: Record<string, unknown> }
    >;
  },
) {
  const now = new Date().toISOString();

  /** Records a request handled by this subgraph (root field or entity ref). */
  const record = (context: any, fieldName: string) => {
    const headers: Record<string, string | undefined> =
      context?.req?.headers ?? {};
    const existing = ctx.captured.find(
      (c) =>
        c.subgraph === name &&
        c.query === context?.query &&
        c.headers === headers,
    );
    const entry = existing ?? {
      subgraph: name,
      query: context?.query,
      variables: context?.variables,
      headers,
      fields: [] as string[],
    };
    if (!existing) ctx.captured.push(entry);
    if (!entry.fields.includes(fieldName)) entry.fields.push(fieldName);
    return entry;
  };

  // Generic resolver factory: records the call, checks for injected errors,
  // and returns a value derived from the field name.
  const root = (
    fieldName: string,
    impl?: (args: any, info: GraphQLResolveInfo) => any,
  ) => {
    return (parent: any, args: any, context: any, info: GraphQLResolveInfo) => {
      record(context, fieldName);

      const errKey = `${name}.${fieldName}`;
      const injected = ctx.errors.get(errKey);
      if (injected) {
        // one-shot: clear after throwing so subsequent tests aren't affected
        ctx.errors.delete(errKey);
        const { GraphQLError } = require('graphql');
        throw new GraphQLError(injected.message, {
          extensions: injected.extensions ?? {
            success: false,
            statusCode: 500,
          },
        });
      }

      if (impl) return impl(args, info);
      return defaultData(name, fieldName, args);
    };
  };

  /** Entity resolver that also records the `_entities` round-trip. */
  const entity = (ref: any, context: any, resolve: (ref: any) => any) => {
    record(context, `_entities:${ref?.__typename ?? 'Entity'}`);
    return resolve(ref);
  };

  const queryFields: Record<string, unknown> = {};
  const mutationFields: Record<string, unknown> = {};

  const QUERY_NAMES: Record<string, string[]> = {
    user: ['pingForUser', 'myProfile', 'user', 'myAddresses'],
    delivery: [
      'deliveryServiceInfo',
      'deliveryNextStatuses',
      'myDeliveries',
      'delivery',
      'openDeliveries',
    ],
    driver: [
      'driverServiceInfo',
      'driver',
      'myDriverProfile',
      'driverStatus',
      'nearbyDrivers',
      'driverActiveAssignment',
      'dispatchAttempts',
      'assignment',
      'availableDeliveries',
      'driverReviews',
    ],
    payment: ['paymentServiceInfo', 'payments', 'payment'],
    notification: [
      'pingForNotification',
      'myNotifications',
      'notification',
      'unreadNotificationCount',
      'myNotificationPreferences',
    ],
    realtime: [
      'pingForRealtime',
      'getConnectionStatus',
      'getActiveConnections',
    ],
    media: [
      'listMedia',
      'media',
      'uploadStatus',
      'downloadUrl',
      'quota',
      'dlqTopics',
      'dlqStats',
    ],
    search: [
      'searchHealth',
      'autocomplete',
      'searchMedia',
      'searchUsers',
      'searchDeliveries',
      'nearbyDeliveries',
      'searchDrivers',
      'searchNearbyDrivers',
    ],
    analytics: [
      'analyticsServiceInfo',
      'platformOverview',
      'deliveryAnalytics',
      'driverAnalytics',
      'topDrivers',
      'paymentAnalytics',
      'rawAnalyticsEvents',
      'dataQualityIssues',
    ],
  };

  const MUTATION_NAMES: Record<string, string[]> = {
    user: [
      'updateProfile',
      'promoteUserToAdmin',
      'toggleUserActive',
      'deleteUser',
      'addAddress',
      'deleteAddress',
      'setDefaultAddress',
      'register',
      'login',
      'changePassword',
      'forgetPassword',
      'resetPassword',
      'refreshToken',
      'logout',
    ],
    delivery: ['createDelivery', 'transitionDelivery', 'cancelDelivery'],
    driver: [
      'registerDriver',
      'updateDriverProfile',
      'activateDriver',
      'goOnline',
      'goOffline',
      'blockDriver',
      'unblockDriver',
      'suspendDriver',
      'deactivateDriver',
      'acceptAssignment',
      'rejectAssignment',
      'selfAcceptDelivery',
      'rateDriver',
    ],
    notification: [
      'markNotificationAsRead',
      'markAllNotificationsAsRead',
      'deleteNotification',
      'updateNotificationPreferences',
    ],
    media: [
      'deleteMedia',
      'abortUpload',
      'createUploadSession',
      'completeUpload',
      'dlqReplay',
      'renewPresigned',
    ],
    search: ['startReindex'],
  };

  for (const f of QUERY_NAMES[name] ?? []) queryFields[f] = root(f);
  for (const f of MUTATION_NAMES[name] ?? []) mutationFields[f] = root(f);

  const resolvers: Record<string, unknown> = { Query: queryFields };
  if (Object.keys(mutationFields).length) resolvers.Mutation = mutationFields;

  // Entity reference resolvers so cross-service entity resolution works.
  // Each `__resolveReference` records the `_entities:<Type>` round-trip so the
  // federation specs can assert the owning subgraph really served the fetch.
  if (name === 'user') {
    resolvers.User = {
      __resolveReference: (ref: { id: string }, context: any) =>
        entity({ ...ref, __typename: 'User' }, context, () => userById(ref.id)),
    };
  }
  if (name === 'driver') {
    resolvers.Driver = {
      __resolveReference: (ref: { id: string }, context: any) =>
        entity({ ...ref, __typename: 'Driver' }, context, () =>
          driverById(ref.id),
        ),
    };
  }
  if (name === 'delivery') {
    resolvers.Delivery = {
      __resolveReference: (ref: { id: string }, context: any) =>
        entity({ ...ref, __typename: 'Delivery' }, context, () =>
          deliveryById(ref.id),
        ),
      // `Delivery.customer` returns a `User` reference. Because `User.deliveries`
      // is ALSO owned by this subgraph, the gateway resolves the nested
      // `customer { deliveries { … } }` inline (no `_entities` hop), so we must
      // return a full User object here — a bare `{__typename, id}` reference
      // stub would leave `deliveries` null.
      customer: (parent: any) => ({
        __typename: 'User',
        id: parent?.customer?.id ?? 'customer-1',
        deliveries: [deliveryById('delivery-1')],
      }),
    };
    // `User.deliveries` is owned by the delivery subgraph (see sdl.ts)
    resolvers.User = {
      __resolveReference: (ref: { id: string }, context: any) =>
        entity({ ...ref, __typename: 'User' }, context, () => ({
          id: ref.id,
          deliveries: [deliveryById('delivery-1')],
        })),
    };
  }

  void now;
  return resolvers;
}

// ─── Canned data helpers (schema-valid shapes) ──────────────────────────────
function serviceInfo(name: string) {
  return { name, version: '1.0.0', status: 'UP' };
}

function pagination(total = 1, page = 1) {
  return { totalItems: total, currentPage: page, nextPage: null };
}

export function userById(id: string) {
  return {
    id,
    firstName: 'Omar',
    lastName: 'Sabry',
    email: 'omar@example.com',
    isActive: true,
    imageUrl: 'https://cdn.example.com/avatar.png',
    role: 'USER',
    phoneNumber: '+201137836162',
    createdAt: '2026-01-01T00:00:00.000Z',
    addresses: [
      {
        id: 'addr-1',
        title: 'Home',
        street: 'El-Galaa Street',
        city: 'Cairo',
        state: 'Cairo Governorate',
        postalCode: '11511',
        latitude: 30.0444,
        longitude: 31.2357,
        isDefault: true,
        createdAt: '2026-01-01T00:00:00.000Z',
      },
    ],
  };
}

export function driverById(id: string) {
  return {
    id,
    status: 'ONLINE',
    vehicleType: 'MOTORCYCLE',
    plateNumber: 'ABC-123',
    capacityKg: 50,
    capabilities: ['STANDARD'],
    serviceArea: 'Cairo',
    isActive: true,
    isBlocked: false,
    rating: 4.8,
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-02T00:00:00.000Z',
    // user is resolved via the user subgraph entity reference
    user: { __typename: 'User', id: 'driver-user-1' },
  };
}

export function deliveryById(id: string) {
  return {
    id,
    status: 'PENDING_PAYMENT',
    paymentStatus: 'PENDING',
    checkoutUrl: 'https://pay.example.com/checkout',
    amount: '25.50',
    currency: 'USD',
    pickedUpAt: null,
    completedAt: null,
    cancelledAt: null,
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
    driver: { __typename: 'Driver', id: 'driver-1' },
    customer: { __typename: 'User', id: 'customer-1' },
    pickupAddress: {
      line1: '1 Main Street',
      line2: 'Floor 3, Apt 12',
      city: 'Cairo',
      state: 'Cairo',
      postalCode: '11511',
      countryCode: 'EG',
      latitude: 30.0444,
      longitude: 31.2357,
    },
    dropoffAddress: {
      line1: '2 Nile Corniche',
      line2: 'Tower A',
      city: 'Giza',
      state: 'Giza',
      postalCode: '12511',
      countryCode: 'EG',
      latitude: 30.0131,
      longitude: 31.2089,
    },
    statusHistory: [
      {
        status: 'PENDING_PAYMENT',
        changedBy: 'system',
        note: 'created',
        createdAt: '2026-01-01T00:00:00.000Z',
      },
    ],
  };
}

function assignmentById(id: string) {
  return {
    id,
    status: 'OFFERED',
    attemptNumber: 1,
    offeredAt: '2026-01-01T00:00:00.000Z',
    expiresAt: '2026-01-01T00:00:30.000Z',
    acceptedAt: null,
    rejectedAt: null,
    completedAt: null,
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
    delivery: { __typename: 'Delivery', id: 'delivery-1' },
    driver: { __typename: 'Driver', id: 'driver-1' },
  };
}

function paymentById(id: string) {
  return {
    id,
    deliveryId: 'delivery-1',
    userId: 'customer-1',
    amountMinor: 2550,
    currency: 'USD',
    status: 'CAPTURED',
    provider: 'STRIPE',
    correlationId: 'corr-1',
    causationId: 'cause-1',
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
    authorizedAt: '2026-01-01T00:00:00.000Z',
    capturedAt: '2026-01-01T00:00:00.000Z',
    delivery: { __typename: 'Delivery', id: 'delivery-1' },
    user: { __typename: 'User', id: 'customer-1' },
  };
}

function mediaItemById(id: string) {
  return {
    mediaId: id,
    ownerId: 'owner-1',
    fileName: 'sample.mp4',
    contentType: 'video/mp4',
    mediaType: 'video',
    size: 104857600,
    objectKey: `media/${id}/original.mp4`,
    status: 'READY',
    createdAt: '2026-01-01T00:00:00.000Z',
    versions: [
      {
        versionType: 'thumbnail',
        objectKey: `media/${id}/thumb.jpg`,
        contentType: 'image/jpeg',
        size: 20480,
        width: 320,
        height: 180,
        durationMs: null,
      },
    ],
  };
}

/** Returns canned, schema-valid data for a root field. */
function defaultData(subgraph: string, field: string, args: any): any {
  switch (`${subgraph}.${field}`) {
    // ── user ──────────────────────────────────────────────────────────────
    case 'user.pingForUser':
      return { statusCode: 200, message: 'ok', success: true, data: true };
    case 'user.myProfile':
    case 'user.updateProfile':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: userById('me-1'),
      };
    case 'user.user':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: userById(args.id),
      };
    case 'user.myAddresses':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: userById('me-1').addresses,
      };
    case 'user.addAddress':
      return {
        statusCode: 201,
        message: 'ok',
        success: true,
        data: {
          id: 'addr-new',
          ...args.input,
          latitude: 30.0,
          longitude: 31.0,
          createdAt: '2026-01-01T00:00:00.000Z',
        },
      };
    case 'user.register':
    case 'user.login':
    case 'user.refreshToken':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: {
          accessToken: 'access-token',
          refreshToken: 'refresh-token',
          user: userById('me-1'),
        },
      };
    case 'user.promoteUserToAdmin':
      return { statusCode: 200, message: 'ok', success: true, data: true };
    case 'user.toggleUserActive':
    case 'user.deleteUser':
    case 'user.deleteAddress':
    case 'user.setDefaultAddress':
    case 'user.changePassword':
    case 'user.forgetPassword':
    case 'user.resetPassword':
    case 'user.logout':
      return { statusCode: 200, message: 'ok', success: true };

    // ── delivery ──────────────────────────────────────────────────────────
    case 'delivery.deliveryServiceInfo':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: serviceInfo('delivery'),
      };
    case 'delivery.createDelivery':
      return {
        statusCode: 201,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: deliveryById('delivery-new'),
      };
    case 'delivery.transitionDelivery':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          ...deliveryById(args.input.deliveryId),
          status: args.input.status,
        },
      };
    case 'delivery.cancelDelivery':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          id: args.deliveryId,
          status: 'CANCELLED',
          cancelledAt: '2026-01-01T00:00:00.000Z',
          updatedAt: '2026-01-01T00:00:00.000Z',
        },
      };
    case 'delivery.deliveryNextStatuses':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: { statuses: ['PAYMENT_CONFIRMED', 'CANCELLED'] },
      };
    case 'delivery.myDeliveries':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1, args.page ?? 1),
          items: [deliveryById('delivery-1')],
        },
      };
    case 'delivery.delivery':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: deliveryById(args.id),
      };
    case 'delivery.openDeliveries':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: {
          items: [deliveryById('delivery-open')],
          paginationInfo: pagination(1, args.page ?? 1),
        },
      };

    // ── driver ────────────────────────────────────────────────────────────
    case 'driver.driverServiceInfo':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: serviceInfo('driver'),
      };
    case 'driver.driver':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: driverById(args.id),
      };
    case 'driver.myDriverProfile':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: driverById('driver-1'),
      };
    case 'driver.registerDriver':
    case 'driver.updateDriverProfile':
    case 'driver.activateDriver':
    case 'driver.goOnline':
    case 'driver.goOffline':
    case 'driver.blockDriver':
    case 'driver.unblockDriver':
    case 'driver.suspendDriver':
    case 'driver.deactivateDriver':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: driverById('driver-1'),
      };
    case 'driver.driverStatus':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: {
          status: 'ONLINE',
          hasActiveAssignment: false,
          activeDelivery: null,
          lastSeenAt: '2026-01-01T00:00:00.000Z',
        },
      };
    case 'driver.nearbyDrivers':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [
            {
              driver: driverById('driver-1'),
              distanceMeters: 1420.5,
              status: 'ONLINE',
              vehicleType: 'MOTORCYCLE',
              latitude: 30.0444,
              longitude: 31.2357,
              rating: 4.8,
              isBlocked: false,
            },
          ],
        },
      };
    case 'driver.driverActiveAssignment':
    case 'driver.assignment':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: assignmentById('assignment-1'),
      };
    case 'driver.acceptAssignment':
    case 'driver.rejectAssignment':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: { ...assignmentById('assignment-1'), status: 'ACCEPTED' },
      };
    case 'driver.dispatchAttempts':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [
            {
              id: 'attempt-1',
              delivery: { __typename: 'Delivery', id: 'delivery-1' },
              driver: { __typename: 'Driver', id: 'driver-1' },
              distanceMeters: 1420.5,
              attemptNumber: 1,
              result: 'OFFERED',
              reason: null,
              createdAt: '2026-01-01T00:00:00.000Z',
            },
          ],
        },
      };
    case 'driver.availableDeliveries':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          totalItems: 1,
          items: [
            {
              id: 'delivery-1',
              customerId: 'customer-1',
              status: 'PENDING_PAYMENT',
              pickupCity: 'Cairo',
              pickupCountry: 'EG',
              dropoffCity: 'Giza',
              dropoffCountry: 'EG',
              amount: '25.50',
              currency: 'USD',
              createdAt: '2026-01-01T00:00:00.000Z',
            },
          ],
        },
      };
    case 'driver.selfAcceptDelivery':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          id: 'assignment-1',
          deliveryId: args.deliveryId,
          driverId: 'driver-1',
          status: 'ACCEPTED',
          offeredAt: '2026-01-01T00:00:00.000Z',
          acceptedAt: '2026-01-01T00:00:00.000Z',
          expiresAt: '2026-01-01T00:00:30.000Z',
        },
      };
    case 'driver.rateDriver':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          id: 'review-1',
          driver: { __typename: 'Driver', id: args.input.driverId },
          user: { __typename: 'User', id: 'customer-1' },
          delivery: { __typename: 'Delivery', id: args.input.deliveryId },
          rating: args.input.rating,
          comment: args.input.comment,
          createdAt: '2026-01-01T00:00:00.000Z',
        },
      };
    case 'driver.driverReviews':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1, args.page ?? 1),
          items: [
            {
              id: 'review-1',
              user: { __typename: 'User', id: 'customer-1' },
              delivery: { __typename: 'Delivery', id: 'delivery-1' },
              rating: 5,
              comment: 'Excellent service!',
              createdAt: '2026-01-01T00:00:00.000Z',
            },
          ],
          averageRating: 5,
          totalReviews: 1,
        },
      };

    // ── payment ───────────────────────────────────────────────────────────
    case 'payment.paymentServiceInfo':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: serviceInfo('payment'),
      };
    case 'payment.payments':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1, args.page ?? 1),
          items: [paymentById('payment-1')],
        },
      };
    case 'payment.payment':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: paymentById(args.id),
      };

    // ── notification ──────────────────────────────────────────────────────
    case 'notification.pingForNotification':
      return { statusCode: 200, message: 'ok', success: true, data: true };
    case 'notification.myNotifications':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: { totalCount: 1, items: [notificationById('notif-1')] },
      };
    case 'notification.notification':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: notificationById(args.id),
      };
    case 'notification.unreadNotificationCount':
      return { statusCode: 200, message: 'ok', success: true, data: 3 };
    case 'notification.myNotificationPreferences':
    case 'notification.updateNotificationPreferences':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        items: [
          {
            id: 'pref-1',
            type: 'DELIVERY_CREATED',
            channel: 'EMAIL',
            enabled: true,
          },
        ],
      };
    case 'notification.markNotificationAsRead':
    case 'notification.markAllNotificationsAsRead':
    case 'notification.deleteNotification':
      return { statusCode: 200, message: 'ok', success: true };

    // ── realtime ──────────────────────────────────────────────────────────
    case 'realtime.pingForRealtime':
      return { statusCode: 200, message: 'ok', success: true, data: true };
    case 'realtime.getConnectionStatus':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: {
          isConnected: true,
          lastSeen: '2026-01-01T00:00:00.000Z',
          connectionCount: 2,
        },
      };
    case 'realtime.getActiveConnections':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        data: { total: 3, byRole: { customers: 1, drivers: 1, admins: 1 } },
      };

    // ── media ─────────────────────────────────────────────────────────────
    case 'media.listMedia':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [mediaItemById('media-1')],
        },
      };
    case 'media.media':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: mediaItemById(args.mediaId),
      };
    case 'media.uploadStatus':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          uploadId: args.uploadId,
          status: 'IN_PROGRESS',
          totalParts: 3,
          completedParts: 1,
          missingParts: [2, 3],
          expiresAt: '2026-01-01T01:00:00.000Z',
        },
      };
    case 'media.downloadUrl':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          url: 'https://s3.example.com/download',
          expiresAt: '2026-01-01T01:00:00.000Z',
          contentType: 'video/mp4',
        },
      };
    case 'media.quota':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          usedBytes: 1024,
          quotaBytes: 10240,
          remainingBytes: 9216,
          activeUploads: 1,
          maxConcurrentUploads: 3,
        },
      };
    case 'media.dlqTopics':
      return ['media-events-dlq'];
    case 'media.dlqStats':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          topic: (args.topics || ['media-events-dlq'])[0],
          messageCount: 2,
        },
      };
    case 'media.deleteMedia':
    case 'media.abortUpload':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: true,
      };
    case 'media.createUploadSession':
    case 'media.renewPresigned':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          mediaId: 'media-1',
          uploadId: args.input.uploadId ?? 'upload-001',
          s3UploadId: 's3-upload-1',
          presignedParts: [
            { partNumber: 1, presignedUrl: 'https://s3.example.com/part1' },
          ],
          partSize: 5242880,
          totalParts: 1,
          expiresAt: '2026-01-01T01:00:00.000Z',
        },
      };
    case 'media.completeUpload':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: { mediaId: 'media-1', status: 'READY' },
      };
    case 'media.dlqReplay':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: { success: true, replayedCount: 5, errors: [] },
      };

    // ── search ────────────────────────────────────────────────────────────
    case 'search.searchHealth':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: { status: 'UP', timestamp: '2026-01-01T00:00:00.000Z' },
      };
    case 'search.autocomplete':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: { suggestions: ['Screen', 'Screenshot'] },
      };
    case 'search.searchMedia':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [
            {
              media_id: 'media-1',
              file_name: 'screen.png',
              mime_type: 'image/png',
              owner_id: 'owner-1',
              size: 1024,
              status: 'READY',
              created_at: '2026-01-01T00:00:00.000Z',
            },
          ],
        },
      };
    case 'search.searchUsers':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [
            {
              id: 'u-1',
              first_name: 'Omar',
              last_name: 'Sabry',
              email: 'omar@example.com',
              is_active: true,
              created_at: '2026-01-01T00:00:00.000Z',
            },
          ],
        },
      };
    case 'search.searchDeliveries':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [searchDeliveryItem('delivery-1')],
        },
      };
    case 'search.nearbyDeliveries':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [
            {
              delivery_id: 'delivery-1',
              customer_id: 'customer-1',
              status: 'PENDING_PAYMENT',
              pickup: {
                city: 'Cairo',
                country: 'EG',
                location: { lat: 30.0444, lon: 31.2357 },
              },
            },
          ],
        },
      };
    case 'search.searchDrivers':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [searchDriverItem('driver-1')],
        },
      };
    case 'search.searchNearbyDrivers':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [
            {
              driver_id: 'driver-1',
              name: 'Omar',
              status: 'ONLINE',
              vehicle_type: 'CAR',
              rating: 4.5,
              location: { lat: 12.0, lon: 10.0 },
            },
          ],
        },
      };
    case 'search.startReindex':
      return {
        statusCode: 202,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          jobId: 'job-1',
          index: args.index,
          status: 'STARTED',
          startedAt: '2026-01-01T00:00:00.000Z',
          completedAt: null,
          error: null,
        },
      };

    // ── analytics ─────────────────────────────────────────────────────────
    case 'analytics.analyticsServiceInfo':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: serviceInfo('analytics'),
      };
    case 'analytics.platformOverview':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          totalDeliveries: 100,
          completedDeliveries: 80,
          cancelledDeliveries: 10,
          failedDeliveries: 5,
          completionRate: 0.8,
          averageDeliveryDurationSeconds: 1800,
          paymentCapturedAmount: 2500,
          refundedAmount: 100,
          driverAcceptanceRate: 0.9,
          dataAsOf: '2026-01-01T00:00:00.000Z',
        },
      };
    case 'analytics.deliveryAnalytics':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          total: 100,
          completed: 80,
          cancelled: 10,
          failed: 5,
          completionRate: 0.8,
          averageDurationSeconds: 1800,
          p50DurationSeconds: 1700,
          p95DurationSeconds: 2500,
          p99DurationSeconds: 3000,
          averageAssignmentTimeSeconds: 120,
          buckets: [
            {
              bucket: '2026-01-01T00:00:00Z',
              total: 10,
              completed: 8,
              cancelled: 1,
              failed: 0,
              avgDurationSeconds: 1750,
            },
          ],
          dataAsOf: '2026-01-01T00:00:00.000Z',
        },
      };
    case 'analytics.driverAnalytics':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          driverId: args.filter?.driverId,
          driver: { __typename: 'Driver', id: args.filter?.driverId },
          offers: 20,
          accepted: 18,
          rejected: 1,
          expired: 1,
          acceptanceRate: 0.9,
          averageResponseTimeMs: 4000,
          completedDeliveries: 15,
          dataAsOf: '2026-01-01T00:00:00.000Z',
        },
      };
    case 'analytics.topDrivers':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1),
          items: [
            {
              driverId: 'driver-1',
              driver: { __typename: 'Driver', id: 'driver-1' },
              offers: 20,
              accepted: 18,
              acceptanceRate: 0.9,
              completedDeliveries: 15,
              dataAsOf: '2026-01-01T00:00:00.000Z',
            },
          ],
        },
      };
    case 'analytics.paymentAnalytics':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          authorizationCount: 100,
          authorizationSuccessRate: 0.98,
          captureCount: 95,
          capturedAmount: 2400,
          refundCount: 3,
          refundedAmount: 90,
          refundRate: 0.03,
          averageProviderLatencyMs: 250,
          dataAsOf: '2026-01-01T00:00:00.000Z',
        },
      };
    case 'analytics.rawAnalyticsEvents':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1, args.page ?? 1),
          items: [
            {
              eventId: 'evt-1',
              eventType: 'DELIVERY_CREATED',
              eventVersion: '1.0',
              aggregateType: 'DELIVERY',
              aggregateId: 'delivery-1',
              producer: 'delivery-service',
              occurredAt: '2026-01-01T00:00:00.000Z',
              ingestedAt: '2026-01-01T00:00:01.000Z',
              correlationId: 'corr-1',
              sourceTopic: 'delivery-events',
            },
          ],
        },
      };
    case 'analytics.dataQualityIssues':
      return {
        statusCode: 200,
        message: 'ok',
        success: true,
        timeStamp: '2026-01-01T00:00:00.000Z',
        data: {
          paginationInfo: pagination(1, args.page ?? 1),
          items: [
            {
              issueId: 'issue-1',
              eventId: 'evt-1',
              issueType: 'MISSING_FIELD',
              aggregateType: 'DELIVERY',
              aggregateId: 'delivery-1',
              detectedAt: '2026-01-01T00:00:00.000Z',
              severity: 'INFO',
              details: 'missing dropoff longitude',
            },
          ],
        },
      };

    default:
      return { statusCode: 200, message: 'ok', success: true };
  }
}

function searchDeliveryItem(id: string) {
  return {
    delivery_id: id,
    customer_id: 'customer-1',
    driver_id: 'driver-1',
    status: 'PENDING_PAYMENT',
    pickup: {
      city: 'Cairo',
      country: 'EG',
      location: { lat: 30.0444, lon: 31.2357 },
    },
    dropoff: {
      city: 'Giza',
      country: 'EG',
      location: { lat: 30.0131, lon: 31.2089 },
    },
    created_at: '2026-01-01T00:00:00.000Z',
    updated_at: '2026-01-01T00:00:00.000Z',
    source_version: '1',
  };
}

function searchDriverItem(id: string) {
  return {
    driver_id: id,
    name: 'Omar',
    status: 'ONLINE',
    vehicle_type: 'CAR',
    rating: 4.5,
    location: { lat: 1.0, lon: 10.0 },
    updated_at: '2026-01-01T00:00:00.000Z',
    source_version: '1',
  };
}

function notificationById(id: string) {
  return {
    id,
    type: 'DELIVERY_CREATED',
    title: 'Delivery created',
    body: 'Your delivery has been created',
    priority: 'HIGH',
    createdAt: '2026-01-01T00:00:00.000Z',
    readAt: null,
    deliveries: [
      {
        id: 'nd-1',
        channel: 'EMAIL',
        status: 'SENT',
        sentAt: '2026-01-01T00:00:00.000Z',
        deliveredAt: '2026-01-01T00:00:01.000Z',
      },
    ],
  };
}

/**
 * Boots a real Federation subgraph server for every subgraph on ephemeral
 * ports and returns a context for asserting header propagation and injecting
 * errors. This makes the gateway perform genuine `IntrospectAndCompose`
 * federation and genuine cross-subgraph HTTP calls.
 */
export async function startMockSubgraphs(): Promise<MockSubgraphContext> {
  const captured: CapturedRequest[] = [];
  const errors = new Map<
    string,
    { message: string; extensions?: Record<string, unknown> }
  >();
  const servers: ApolloServer[] = [];
  const urls: Record<string, string> = {};

  for (const [name, sdl] of Object.entries(SUBGRAPH_SDL)) {
    const schema = buildSubgraphSchema({
      typeDefs: parse(sdl),
      // `buildResolvers` returns a deliberately loose map (canned data for
      // every field of every subgraph), which Apollo narrows at runtime.
      resolvers: buildResolvers(name, { captured, errors }) as any,
    });

    const server = new ApolloServer({ schema });
    const { url } = await startStandaloneServer(server, {
      listen: { port: 0 },
      context: async ({ req }: any) => ({
        req,
        query: req.body?.query,
        variables: req.body?.variables,
      }),
    });

    servers.push(server);
    // `startStandaloneServer` always serves GraphQL at `/`, so the gateway must
    // be pointed at the server root URL (not SUBGRAPH_PATH, which only exists
    // when a service mounts its own express router).
    urls[name] = url;
  }

  const ctx: MockSubgraphContext = {
    captured,
    urls,
    forSubgraph: (n) => captured.filter((c) => c.subgraph === n),
    last: (n) => [...captured].reverse().find((c) => c.subgraph === n),
    clear: () => {
      captured.length = 0;
    },
    injectError: (subgraph, field, error) => {
      errors.set(`${subgraph}.${field}`, error);
    },
    clearErrors: () => errors.clear(),
    stop: async () => {
      await Promise.all(servers.map((s) => s.stop()));
    },
  };

  return ctx;
}
