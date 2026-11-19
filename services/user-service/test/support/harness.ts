/**
 * Shared bootstrap / helpers for the end-to-end suite.
 *
 * Responsibilities:
 *  - `ensureDatabase()`  creates the (empty) Postgres database once per run.
 *  - `bootstrap()`       compiles `TestAppModule`, applies the production
 *                        `ValidationPipe` configuration and starts the HTTP app.
 *  - `resetState()`      truncates the tables and flushes the in-memory Redis
 *                        between tests so that "first user becomes ADMIN" and
 *                        the per-IP rate limits stay deterministic.
 *  - `shutdown()`        tears everything down so jest can exit by itself.
 *  - `gql()` / mutations / seed helpers keep the specs declarative.
 */

import { INestApplication, ValidationPipe } from '@nestjs/common';
import { Test, TestingModule } from '@nestjs/testing';
import { DataSource } from 'typeorm';
import request from 'supertest';
import { I18nValidationException } from 'nestjs-i18n';
import { Client } from 'pg';
import { randomUUID } from 'crypto';
import { RedisService, NotificationService } from '@bts-soft/core';
import { KafkaService, Role } from '@delivery-micro/shard';
import { TestAppModule } from '../test-app.module';
import { FakeRedisService } from './fake-redis';
import {
  KafkaFake,
  NotificationFake,
  OutboxFake,
  RabbitPublisherFake,
  MediaFake,
} from './fakes';
import { OutboxWorkerService } from '../../src/common/messaging/outbox-worker.service';
import { MediaGrpcService } from '../../src/modules/media/grpc-media.service';
import { UserRabbitMQPublisher } from '../../src/modules/rabbitmq/rabbitmq.publisher';
import { User } from '../../src/common/database/entities/user.entity';
import { BcryptPasswordHasher } from '../../src/common/security/bcrypt-password.hasher';
import { E2E_DB } from './test-env';

export interface E2EApp {
  app: INestApplication;
  module: TestingModule;
  dataSource: DataSource;
  redis: FakeRedisService;
  kafka: KafkaFake;
  notification: NotificationFake;
  outbox: OutboxFake;
  rabbit: RabbitPublisherFake;
  media: MediaFake;
}

/**
 * Create the target database when it does not exist yet. Connecting to the
 * maintenance database `postgres` and issuing a conditional `CREATE DATABASE`
 * keeps the suite independent of the `psql` client binary.
 */
export async function ensureDatabase(): Promise<void> {
  const client = new Client({
    host: E2E_DB.host,
    port: E2E_DB.port,
    user: E2E_DB.user,
    password: E2E_DB.password,
    database: 'postgres',
  });
  await client.connect();
  try {
    const result = await client.query(
      'SELECT 1 FROM pg_database WHERE datname = $1',
      [E2E_DB.database],
    );
    if ((result.rowCount ?? 0) === 0) {
      const safeName = E2E_DB.database.replace(/"/g, '""');
      await client.query(`CREATE DATABASE "${safeName}"`);
    }
  } finally {
    await client.end().catch(() => undefined);
  }
}

/** Compile the testing module and boot the HTTP app exactly like `main.ts`. */
export async function bootstrap(): Promise<E2EApp> {
  await ensureDatabase();

  const module = await Test.createTestingModule({
    imports: [TestAppModule],
  }).compile();

  const app = module.createNestApplication();

  // Same pipe configuration as src/main.ts — including the I18n exception
  // factory, whose GraphQL error envelope is asserted by the error specs.
  app.useGlobalPipes(
    new ValidationPipe({
      transform: true,
      whitelist: true,
      stopAtFirstError: true,
      exceptionFactory: (errors) => new I18nValidationException(errors),
    }),
  );

  await app.init();

  return {
    app,
    module,
    dataSource: module.get(DataSource),
    redis: module.get(RedisService) as unknown as FakeRedisService,
    kafka: module.get(KafkaService) as unknown as KafkaFake,
    notification: module.get(
      NotificationService,
    ) as unknown as NotificationFake,
    outbox: module.get(OutboxWorkerService) as unknown as OutboxFake,
    rabbit: module.get(UserRabbitMQPublisher) as unknown as RabbitPublisherFake,
    media: module.get(MediaGrpcService) as unknown as MediaFake,
  };
}

/** Close the app (and its data source / watchers) so jest can exit cleanly. */
export async function shutdown(ctx: E2EApp | undefined): Promise<void> {
  if (!ctx) return;
  try {
    if (ctx.dataSource?.isInitialized) {
      await ctx.dataSource.destroy();
    }
  } catch {
    // ignore: the data source may already be gone
  }
  await ctx.app.close();
}

/**
 * Wipe database state and in-memory Redis state (sessions, user cache and
 * rate-limit counters) plus the recorded outbound calls.
 */
export async function resetState(ctx: E2EApp): Promise<void> {
  await ctx.dataSource.query('TRUNCATE TABLE addresses, outbox, users CASCADE');
  ctx.redis.flush();
  ctx.kafka.emit.mockClear();
  ctx.kafka.consumer.mockClear();
  ctx.notification.send.mockClear();
  ctx.outbox.enqueueEvent.mockClear();
  ctx.outbox.reconcileOutbox.mockClear();
  ctx.rabbit.publishUserCreated.mockClear();
  ctx.rabbit.publishUserUpdated.mockClear();
  ctx.rabbit.publishUserDeleted.mockClear();
  ctx.media.resolveMediaUrl.mockClear();
  ctx.media.resolveMediaUrl.mockImplementation(() =>
    Promise.resolve({
      url: 'https://media.test/avatars/resolved.png',
      expiresAtSeconds: 3600,
      contentType: 'image/png',
      mediaId: 'media-1',
      status: 'READY',
    }),
  );
}

// ---------------------------------------------------------------- GraphQL

export interface GqlOptions {
  query: string;
  variables?: Record<string, unknown>;
  headers?: Record<string, string>;
}

/** POST a GraphQL document to the federated `/user/graphql` endpoint. */
export function gql(app: INestApplication, options: GqlOptions) {
  let r = request(app.getHttpServer())
    .post('/user/graphql')
    .set('Content-Type', 'application/json');
  for (const [key, value] of Object.entries(options.headers ?? {})) {
    r = r.set(key, value);
  }
  return r.send({ query: options.query, variables: options.variables });
}

/** Authorization header helper for a Bearer access token. */
export function auth(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}

/** Locale helper: sends `Accept-Language` (or `x-lang`) with a request. */
export function lang(value: string): Record<string, string> {
  return { 'Accept-Language': value, 'x-lang': value };
}

export const addressFragments = `
  fragment AddressFields on AddressType {
    id userId title street city state postalCode latitude longitude isDefault createdAt
  }
`;

export const fragments = `
  ${addressFragments}
  fragment UserFields on User {
    id email firstName lastName role phoneNumber isActive createdAt imageUrl
    addresses { ...AddressFields }
  }
`;

export const MUTATIONS = {
  register: `
    ${fragments}
    mutation Register($input: RegisterInput!) {
      register(input: $input) {
        success statusCode message
        data { accessToken refreshToken user { ...UserFields } }
      }
    }`,
  login: `
    ${fragments}
    mutation Login($input: LoginInput!) {
      login(input: $input) {
        success statusCode message
        data { accessToken refreshToken user { ...UserFields } }
      }
    }`,
  forgetPassword: `
    mutation Forget($input: ForgetPasswordInput!) {
      forgetPassword(input: $input) { success statusCode message data }
    }`,
  resetPassword: `
    mutation Reset($input: ResetPasswordInput!) {
      resetPassword(input: $input) { success statusCode message data }
    }`,
  refreshToken: `
    ${fragments}
    mutation Refresh($input: RefreshTokenInput!) {
      refreshToken(input: $input) {
        success statusCode message
        data { accessToken refreshToken user { ...UserFields } }
      }
    }`,
  logout: `
    mutation Logout { logout { success statusCode message data } }`,
  changePassword: `
    mutation ChangePassword($input: ChangePasswordInput!) {
      changePassword(input: $input) { success statusCode message data }
    }`,
  updateProfile: `
    ${fragments}
    mutation UpdateProfile($input: UpdateProfileInput!) {
      updateProfile(input: $input) {
        success statusCode message
        data { ...UserFields }
      }
    }`,
  promoteUserToAdmin: `
    mutation Promote($id: String!) {
      promoteUserToAdmin(id: $id) { success statusCode message data }
    }`,
  toggleUserActive: `
    mutation Toggle($id: String!, $isActive: Boolean!) {
      toggleUserActive(id: $id, isActive: $isActive) { success statusCode message data }
    }`,
  deleteUser: `
    mutation Delete($id: String!) {
      deleteUser(id: $id) { success statusCode message data }
    }`,
  addAddress: `
    ${addressFragments}
    mutation AddAddress($input: AddAddressInput!) {
      addAddress(input: $input) { success statusCode message data { ...AddressFields } }
    }`,
  deleteAddress: `
    mutation DeleteAddress($addressId: String!) {
      deleteAddress(addressId: $addressId) { success statusCode message data }
    }`,
  setDefaultAddress: `
    mutation SetDefault($addressId: String!) {
      setDefaultAddress(addressId: $addressId) { success statusCode message data }
    }`,
};

export const QUERIES = {
  pingForUser: `query { pingForUser { success statusCode message data } }`,
  myProfile: `
    ${fragments}
    query MyProfile { myProfile { success statusCode message data { ...UserFields } } }`,
  user: `
    ${fragments}
    query GetUser($id: String!) { user(id: $id) { success statusCode message data { ...UserFields } } }`,
  myAddresses: `
    ${addressFragments}
    query MyAddresses { myAddresses { success statusCode message data { ...AddressFields } } }`,
  service: `query { _service { sdl } }`,
  entities: `
    query Entities($representations: [_Any!]!) {
      _entities(representations: $representations) {
        ... on User { id email firstName lastName role }
      }
    }`,
};

export interface RegisterOverrides {
  email?: string;
  password?: string;
  firstName?: string;
  lastName?: string;
  phoneNumber?: string;
  imageUrl?: string;
}

/** Build a valid `RegisterInput` (unique e-mail + unique Egyptian phone). */
let seq = 0;
const egPhone = () => `01${String(200000000 + seq).slice(-9)}`;

/** Unique Egyptian mobile number (also the `seedUser` default). */
export function nextPhone(): string {
  seq += 1;
  return egPhone();
}

export function registerInput(overrides: RegisterOverrides = {}) {
  // Every call burns a sequence number, so the default e-mail and the default
  // phone stay unique even when the caller overrides only one of them.
  seq += 1;
  return {
    email: overrides.email ?? `user${seq}@example.com`,
    password: overrides.password ?? 'Passw0rd1',
    firstName: overrides.firstName ?? 'John',
    lastName: overrides.lastName ?? 'Doe',
    // `phoneNumber` is `String!` in the GraphQL schema, so it always ships.
    phoneNumber: overrides.phoneNumber ?? egPhone(),
    ...(overrides.imageUrl ? { imageUrl: overrides.imageUrl } : {}),
  };
}

/** Register a user through GraphQL and return the parsed payload. */
export async function registerUser(
  ctx: E2EApp,
  overrides: RegisterOverrides = {},
) {
  const input = registerInput(overrides);
  const res = await gql(ctx.app, {
    query: MUTATIONS.register,
    variables: { input },
  });
  return { input, res, body: res.body?.data?.register };
}

/** Log in through GraphQL and return the auth payload. */
export async function loginUser(
  ctx: E2EApp,
  email: string,
  password = 'Passw0rd1',
) {
  const res = await gql(ctx.app, {
    query: MUTATIONS.login,
    variables: { input: { email, password } },
  });
  return { res, body: res.body?.data?.login };
}

// ------------------------------------------------------------------ Seeds

export interface SeedOptions {
  email: string;
  password?: string;
  role?: Role;
  phone?: string;
  firstName?: string;
  lastName?: string;
  isActive?: boolean;
}

/**
 * Insert a user straight into Postgres. Used by specs that need more accounts
 * than the per-IP `register` rate limit (3/min) allows, or that must own a
 * known password before hitting `login` (5/min).
 */
export async function seedUser(
  dataSource: DataSource,
  options: SeedOptions,
): Promise<User> {
  const user = new User();
  user.id = randomUUID();
  user.email = options.email.toLowerCase();
  user.passwordHash = await new BcryptPasswordHasher().hash(
    options.password ?? 'Passw0rd1',
  );
  user.firstName = options.firstName ?? 'Test';
  user.lastName = options.lastName ?? 'User';
  user.phoneNumber = options.phone ?? nextPhone();
  user.role = options.role ?? Role.USER;
  user.isActive = options.isActive ?? true;
  user.addresses = [];
  return dataSource.getRepository(User).save(user);
}

/** Read a translation file directly so specs never hardcode message text. */
export function loadLocale(langCode: 'en' | 'ar'): Record<string, string> {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  return require(`../../src/common/translation/locales/${langCode}/user.json`);
}
