import { Test, TestingModule } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import * as request from 'supertest';
import { TestAppModule } from './test-app.module';
import { DataSource } from 'typeorm';

describe('UserService E2E Tests', () => {
  let app: INestApplication;
  let dataSource: DataSource;

  beforeAll(async () => {
    const moduleFixture: TestingModule = await Test.createTestingModule({
      imports: [TestAppModule],
    }).compile();

    app = moduleFixture.createNestApplication();
    await app.init();

    dataSource = moduleFixture.get<DataSource>(DataSource);
  }, 60000);

  afterAll(async () => {
    if (dataSource && dataSource.isInitialized) {
      await dataSource.destroy();
    }
    await app.close();
  }, 10000);

  describe('Health & Metrics Endpoints', () => {
    describe('GET /user/health', () => {
      it('should return health status', async () => {
        const response = await request(app.getHttpServer())
          .get('/user/health')
          .expect(200);

        expect(response.body).toHaveProperty('status', 'UP');
        expect(response.body).toHaveProperty('service', 'user-service');
        expect(response.body).toHaveProperty('timestamp');
        expect(response.body).toHaveProperty('system');
      });
    });

    describe('GET /user/ready', () => {
      it('should return readiness status', async () => {
        const response = await request(app.getHttpServer())
          .get('/user/ready')
          .expect(200);

        expect(response.body).toHaveProperty('status', 'UP');
        expect(response.body).toHaveProperty('timestamp');
        expect(response.body).toHaveProperty('checks');
        expect(response.body.checks).toHaveProperty('database');
        expect(response.body.checks).toHaveProperty('redis');
      });
    });

    describe('GET /user/metrics', () => {
      it('should return Prometheus metrics', async () => {
        const response = await request(app.getHttpServer())
          .get('/user/metrics')
          .expect(200);

        expect(response.text).toContain('# HELP');
        expect(response.text).toContain('# TYPE');
      });
    });
  });

  describe('GraphQL Playground', () => {
    it('should serve GraphQL playground', async () => {
      const response = await request(app.getHttpServer())
        .get('/user/graphql')
        .expect(200);

      expect(response.text).toContain('GraphQL Playground');
    });
  });
});
