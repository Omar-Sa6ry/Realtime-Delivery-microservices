/**
 * Analytics service operations — mirrors graphql-docs/analytics.graphql.
 */
import { createGatewayHarness, GatewayHarness } from './support/harness';

jest.setTimeout(120_000);

const RANGE = `{ from: "2026-09-12T00:00:00Z", to: "2026-10-19T23:59:59Z", granularity: DAY }`;
const PAGINATION = `paginationInfo { totalItems currentPage nextPage }`;

describe('Analytics service (graphql-docs/analytics.graphql)', () => {
  let h: GatewayHarness;
  let token: string;

  beforeAll(async () => {
    h = await createGatewayHarness();
    token = h.signToken({
      sub: 'admin-1',
      role: 'ADMIN',
      sessionId: 'sess-a1',
    });
  });

  afterAll(async () => {
    await h.close();
  });

  describe('analyticsServiceInfo (Public)', () => {
    it('returns service metadata', async () => {
      const res = await h
        .gql(
          `query GetAnalyticsServiceInfo {
            analyticsServiceInfo { success statusCode message timeStamp data { name version status } }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.analyticsServiceInfo.data).toMatchObject({
        name: 'analytics',
        version: '1.0.0',
        status: 'UP',
      });
    });
  });

  describe('platformOverview (Admin)', () => {
    it('summarises platform KPIs for a range', async () => {
      const res = await h
        .gql(
          `query GetPlatformOverview {
            platformOverview(range: ${RANGE}) {
              success statusCode message timeStamp
              data {
                totalDeliveries completedDeliveries cancelledDeliveries failedDeliveries
                completionRate averageDeliveryDurationSeconds paymentCapturedAmount
                refundedAmount driverAcceptanceRate dataAsOf
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.platformOverview.data).toEqual({
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
      });
    });

    it('requires the range argument', async () => {
      const res = await h
        .gql(`{ platformOverview { statusCode } }`)
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0]).toMatchObject({
        success: false,
        statusCode: 400,
      });
    });
  });

  describe('deliveryAnalytics (Admin)', () => {
    it('returns percentile metrics and time buckets', async () => {
      const res = await h
        .gql(
          `query GetDeliveryAnalytics {
            deliveryAnalytics(filter: { range: ${RANGE} }) {
              success statusCode message timeStamp
              data {
                total completed cancelled failed completionRate
                averageDurationSeconds p50DurationSeconds p95DurationSeconds p99DurationSeconds
                averageAssignmentTimeSeconds
                buckets { bucket total completed cancelled failed avgDurationSeconds }
                dataAsOf
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.deliveryAnalytics.data;
      expect(data).toMatchObject({
        total: 100,
        completed: 80,
        completionRate: 0.8,
        p50DurationSeconds: 1700,
        p95DurationSeconds: 2500,
        p99DurationSeconds: 3000,
      });
      expect(data.buckets[0]).toMatchObject({
        bucket: '2026-01-01T00:00:00Z',
        total: 10,
        completed: 8,
      });
    });

    it('accepts an optional driverId and status filter', async () => {
      const res = await h
        .gql(
          `{
            deliveryAnalytics(filter: { range: ${RANGE}, driverId: "driver-1", status: "COMPLETED" }) {
              success data { total }
            }
          }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deliveryAnalytics.data.total).toBe(100);
    });
  });

  describe('driverAnalytics (Driver / Admin)', () => {
    it('returns per-driver acceptance metrics with the federated driver', async () => {
      const res = await h
        .gql(
          `query GetDriverAnalytics($driverId: ID!) {
            driverAnalytics(filter: { range: ${RANGE}, driverId: $driverId }) {
              success statusCode message timeStamp
              data {
                driverId driver { id }
                offers accepted rejected expired acceptanceRate
                averageResponseTimeMs completedDeliveries dataAsOf
              }
            }
          }`,
          { driverId: 'e8b7e043-a5ca-4753-b24d-43587c507ab7' },
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const data = res.body.data.driverAnalytics.data;
      expect(data).toMatchObject({
        driverId: 'e8b7e043-a5ca-4753-b24d-43587c507ab7',
        offers: 20,
        accepted: 18,
        rejected: 1,
        expired: 1,
        acceptanceRate: 0.9,
        averageResponseTimeMs: 4000,
        completedDeliveries: 15,
      });
      // Driver entity resolved by the driver subgraph
      expect(data.driver).toEqual({
        id: 'e8b7e043-a5ca-4753-b24d-43587c507ab7',
      });
    });
  });

  describe('topDrivers (Admin)', () => {
    it('ranks drivers with federated entities', async () => {
      const res = await h
        .gql(
          `query GetTopDrivers {
            topDrivers(range: ${RANGE}, limit: 10) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items { driverId driver { id } offers accepted acceptanceRate completedDeliveries dataAsOf }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      const items = res.body.data.topDrivers.data.items;
      expect(items[0]).toMatchObject({
        driverId: 'driver-1',
        offers: 20,
        accepted: 18,
        acceptanceRate: 0.9,
        completedDeliveries: 15,
      });
      expect(items[0].driver).toEqual({ id: 'driver-1' });
    });
  });

  describe('paymentAnalytics (Admin)', () => {
    it('returns payment KPIs filtered by provider', async () => {
      const res = await h
        .gql(
          `query GetPaymentAnalytics {
            paymentAnalytics(filter: { range: ${RANGE}, provider: STRIPE }) {
              success statusCode message timeStamp
              data {
                authorizationCount authorizationSuccessRate captureCount capturedAmount
                refundCount refundedAmount refundRate averageProviderLatencyMs dataAsOf
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.paymentAnalytics.data).toMatchObject({
        authorizationCount: 100,
        authorizationSuccessRate: 0.98,
        captureCount: 95,
        capturedAmount: 2400,
        refundCount: 3,
        refundRate: 0.03,
      });
    });

    it('rejects an unknown provider enum', async () => {
      const res = await h
        .gql(
          `{ paymentAnalytics(filter: { range: ${RANGE}, provider: BITCOIN }) { statusCode } }`,
        )
        .expect(400);

      expect(res.body.errors).toBeDefined();
      expect(res.body.errors[0].message).toMatch(/BITCOIN|PaymentProvider/);
    });
  });

  describe('rawAnalyticsEvents (Admin)', () => {
    it('lists raw events filtered by type', async () => {
      const res = await h
        .gql(
          `query GetRawAnalyticsEvents {
            rawAnalyticsEvents(page: 1, limit: 20, eventType: DELIVERY_CREATED) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items {
                  eventId eventType eventVersion aggregateType aggregateId producer
                  occurredAt ingestedAt correlationId sourceTopic
                }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.rawAnalyticsEvents.data.items[0]).toMatchObject({
        eventId: 'evt-1',
        eventType: 'DELIVERY_CREATED',
        aggregateType: 'DELIVERY',
        producer: 'delivery-service',
        sourceTopic: 'delivery-events',
      });
    });

    it('works without the eventType filter', async () => {
      const res = await h
        .gql(
          `{ rawAnalyticsEvents(page: 1, limit: 5) { success data { ${PAGINATION} } } }`,
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(
        res.body.data.rawAnalyticsEvents.data.paginationInfo.totalItems,
      ).toBe(1);
    });
  });

  describe('dataQualityIssues (Admin)', () => {
    it('lists issues filtered by severity', async () => {
      const res = await h
        .gql(
          `query GetDataQualityIssues {
            dataQualityIssues(page: 1, limit: 20, severity: INFO) {
              success statusCode message timeStamp
              data {
                ${PAGINATION}
                items { issueId eventId issueType aggregateType aggregateId detectedAt severity details }
              }
            }
          }`,
          undefined,
          { token },
        )
        .expect(200);

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.dataQualityIssues.data.items[0]).toMatchObject({
        issueId: 'issue-1',
        issueType: 'MISSING_FIELD',
        severity: 'INFO',
        details: 'missing dropoff longitude',
      });
    });
  });

  it('routes every analytics operation to the analytics subgraph', async () => {
    h.subgraphs.clear();

    await h.gql(`{ analyticsServiceInfo { success } }`).expect(200);
    await h
      .gql(`{ platformOverview(range: ${RANGE}) { success } }`)
      .expect(200);
    await h
      .gql(`{ deliveryAnalytics(filter: { range: ${RANGE} }) { success } }`)
      .expect(200);
    await h
      .gql(
        `{ driverAnalytics(filter: { range: ${RANGE}, driverId: "d1" }) { success } }`,
      )
      .expect(200);
    await h.gql(`{ topDrivers(range: ${RANGE}) { success } }`).expect(200);
    await h
      .gql(`{ paymentAnalytics(filter: { range: ${RANGE} }) { success } }`)
      .expect(200);
    await h.gql(`{ rawAnalyticsEvents { success } }`).expect(200);
    await h.gql(`{ dataQualityIssues { success } }`).expect(200);

    const fields = new Set(
      h.subgraphs.forSubgraph('analytics').flatMap((r) => r.fields),
    );
    for (const f of [
      'analyticsServiceInfo',
      'platformOverview',
      'deliveryAnalytics',
      'driverAnalytics',
      'topDrivers',
      'paymentAnalytics',
      'rawAnalyticsEvents',
      'dataQualityIssues',
    ]) {
      expect(fields).toContain(f);
    }
  });
});
