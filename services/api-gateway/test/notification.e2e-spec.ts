/**
 * Notification service operations — mirrors graphql-docs/notification.graphql.
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const NOTIFICATION_FIELDS = `
  id
  type
  title
  createdAt
  body
  priority
  readAt
  deliveries { id channel status sentAt deliveredAt }
`;

describe('Notification service (graphql-docs/notification.graphql)', () => {
  let h: GatewayHarness;
  let token: string;

  beforeAll(async () => {
    h = await createGatewayHarness();
    token = h.signToken({ sub: 'user-1', role: 'USER', sessionId: 'sess-1' });
  });

  afterAll(async () => {
    await h.close();
  });

  describe('pingForNotification (Public)', () => {
    it('answers the documented query', async () => {
      const res = await h
        .gql(`{ pingForNotification { statusCode message success data } }`)
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.pingForNotification).toMatchObject({
        statusCode: 200,
        message: 'ok',
        success: true,
        data: true,
      });
    });
  });

  describe('myNotifications (User / Driver / Admin)', () => {
    it('lists notifications with channel deliveries', async () => {
      const res = await h
        .gql(
          `{ myNotifications {
            statusCode message success
            data {
              totalCount
              items { ${NOTIFICATION_FIELDS} }
            }
          } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.myNotifications.data;
      expect(data.totalCount).toBe(1);
      expect(data.items[0]).toMatchObject({
        id: 'notif-1',
        type: 'DELIVERY_CREATED',
        title: 'Delivery created',
        body: 'Your delivery has been created',
        priority: 'HIGH',
        readAt: null,
      });
      expect(data.items[0].deliveries[0]).toMatchObject({
        id: 'nd-1',
        channel: 'EMAIL',
        status: 'SENT',
      });
      expect(h.subgraphs.last('notification')?.headers['x-user-id']).toBe(
        'user-1',
      );
    });
  });

  describe('notification(id) (User / Driver / Admin)', () => {
    it('fetches one notification', async () => {
      const res = await h
        .gql(
          `{ notification(id: "365065336560553984") {
            statusCode message success data { ${NOTIFICATION_FIELDS} }
          } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.notification.data.id).toBe('365065336560553984');
      expect(res.body.data.notification.data.deliveries).toHaveLength(1);
    });
  });

  describe('notification mutations (User / Driver / Admin)', () => {
    it('markNotificationAsRead', async () => {
      const res = await h
        .gql(
          `mutation { markNotificationAsRead(id: "365065336560553984") { statusCode message success } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.markNotificationAsRead).toMatchObject({
        statusCode: 200,
        success: true,
      });
      expect(h.subgraphs.last('notification')?.fields).toContain(
        'markNotificationAsRead',
      );
    });

    it('markAllNotificationsAsRead takes no arguments', async () => {
      const res = await h
        .gql(
          `mutation { markAllNotificationsAsRead { statusCode message success } }`,
          undefined,
          {
            token,
          },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.markAllNotificationsAsRead).toMatchObject({
        success: true,
      });
    });

    it('deleteNotification', async () => {
      const res = await h
        .gql(
          `mutation { deleteNotification(id: "365065517569937408") { statusCode message success } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deleteNotification).toMatchObject({ success: true });
    });
  });

  describe('unreadNotificationCount (User / Driver / Admin)', () => {
    it('returns the unread count', async () => {
      const res = await h
        .gql(
          `{ unreadNotificationCount { statusCode message success data } }`,
          undefined,
          {
            token,
          },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.unreadNotificationCount).toMatchObject({
        statusCode: 200,
        success: true,
        data: 3,
      });
    });
  });

  describe('notification preferences (User / Driver / Admin)', () => {
    it('myNotificationPreferences lists channel preferences', async () => {
      const res = await h
        .gql(
          `{ myNotificationPreferences {
            statusCode message success
            items { id type channel enabled }
          } }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.myNotificationPreferences.items[0]).toEqual({
        id: 'pref-1',
        type: 'DELIVERY_CREATED',
        channel: 'EMAIL',
        enabled: true,
      });
    });

    it('updateNotificationPreferences accepts the documented enum payload', async () => {
      const res = await h
        .gql(
          `mutation {
            updateNotificationPreferences(preferences: [
              {
                type: DELIVERY_CREATED
                channels: [
                  { channel: SMS, enabled: false }
                  { channel: EMAIL, enabled: true }
                ]
              }
            ]) {
              statusCode message success
              items { id type channel enabled }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.updateNotificationPreferences.items).toHaveLength(1);
    });

    it('rejects an unknown notification type', async () => {
      const res = await h
        .gql(
          `mutation { updateNotificationPreferences(preferences: [{ type: NOT_A_TYPE, channels: [] }]) { statusCode } }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toMatch(/NOT_A_TYPE|NotificationType/);
    });
  });
});
