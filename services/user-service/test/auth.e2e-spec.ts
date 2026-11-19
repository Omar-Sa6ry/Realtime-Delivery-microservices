import { ChannelType } from '@bts-soft/core';
import { UserKafkaTopics } from '@delivery-micro/shard';
import { User } from '../src/common/database/entities/user.entity';
import { Outbox } from '../src/common/database/entities/outbox.entity';
import { BcryptPasswordHasher } from '../src/common/security/bcrypt-password.hasher';
import {
  E2EApp,
  bootstrap,
  shutdown,
  resetState,
  gql,
  MUTATIONS,
  QUERIES,
  registerUser,
  loginUser,
  seedUser,
  auth,
  loadLocale,
  nextPhone,
} from './support/harness';

const en = loadLocale('en');

/**
 * Registration, login, logout, token refresh and the password-reset flow,
 * including input validation, the per-IP rate limits and every outbound
 * side effect (outbox row + BullMQ enqueue, Kafka event, RabbitMQ publish,
 * Redis session, notification e-mail).
 */
describe('Authentication flows', () => {
  let ctx: E2EApp;

  beforeAll(async () => {
    ctx = await bootstrap();
  }, 60000);

  afterAll(async () => {
    await shutdown(ctx);
  }, 30000);

  beforeEach(async () => {
    await resetState(ctx);
  });

  const users = () => ctx.dataSource.getRepository(User);
  const outbox = () => ctx.dataSource.getRepository(Outbox);
  const hasher = new BcryptPasswordHasher();

  describe('register', () => {
    it('makes the very first account an ADMIN and returns a full session', async () => {
      const { res, body } = await registerUser(ctx, {
        email: 'first@example.com',
        phoneNumber: nextPhone(),
      });

      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();
      expect(body.success).toBe(true);
      expect(body.statusCode).toBe(201);
      expect(body.message).toBe(en.REGISTER_SUCCESS);

      const user = body.data.user;
      expect(user.role).toBe('admin');
      expect(user.email).toBe('first@example.com');
      expect(user.firstName).toBe('John');
      expect(user.lastName).toBe('Doe');
      expect(user.isActive).toBe(true);
      expect(typeof user.createdAt).toBe('string');
      expect(user.addresses).toEqual([]);
      expect(typeof body.data.accessToken).toBe('string');
      expect(typeof body.data.refreshToken).toBe('string');
      expect(body.data.accessToken.split('.')).toHaveLength(3);

      // persisted + hashed password
      const rows = await users().find();
      expect(rows).toHaveLength(1);
      expect(rows[0].role).toBe('admin');
      expect(rows[0].passwordHash).not.toBe('Passw0rd1');
      await expect(
        hasher.compare('Passw0rd1', rows[0].passwordHash),
      ).resolves.toBe(true);

      // transactional outbox row + BullMQ dispatch
      const events = await outbox().find();
      expect(events).toHaveLength(1);
      expect(events[0].eventType).toBe('user.registered');
      expect(events[0].aggregateType).toBe('User');
      expect(events[0].processed).toBe(false);
      expect(events[0].payload).toMatchObject({
        userId: rows[0].id,
        email: 'first@example.com',
        role: 'admin',
      });
      expect(ctx.outbox.enqueueEvent).toHaveBeenCalledTimes(1);
      expect(ctx.outbox.enqueueEvent).toHaveBeenCalledWith(events[0].id);

      // Kafka + RabbitMQ fan-out
      expect(ctx.kafka.emit).toHaveBeenCalledWith(
        UserKafkaTopics.USER_CREATED,
        UserKafkaTopics.USER_CREATED,
        expect.objectContaining({ userId: rows[0].id, role: 'admin' }),
      );
      expect(ctx.rabbit.publishUserCreated).toHaveBeenCalledWith(
        expect.objectContaining({ id: rows[0].id, email: 'first@example.com' }),
      );

      // Redis session for the issued tokens
      expect(await ctx.redis.keys(`user:session:${rows[0].id}:*`)).toHaveLength(
        1,
      );
    });

    it('creates every later account with role USER and a normalized e-mail', async () => {
      const first = await registerUser(ctx, { email: 'admin@example.com' });
      const second = await registerUser(ctx, {
        email: 'SECOND@Example.COM',
        firstName: 'jane',
        lastName: 'smith',
      });

      expect(first.body.data.user.role).toBe('admin');
      expect(second.res.body.errors).toBeUndefined();
      expect(second.body.success).toBe(true);
      expect(second.body.statusCode).toBe(201);

      // e-mail lower-cased, names capitalized by the validation transforms
      expect(second.body.data.user.email).toBe('second@example.com');
      expect(second.body.data.user.firstName).toBe('Jane');
      expect(second.body.data.user.lastName).toBe('Smith');
      expect(second.body.data.user.role).toBe('user');

      const rows = await users().find();
      expect(rows).toHaveLength(2);
      expect(rows.map((r) => r.email).sort()).toEqual([
        'admin@example.com',
        'second@example.com',
      ]);
      const secondRow = await users().findOne({
        where: { email: 'second@example.com' },
      });
      expect(secondRow?.role).toBe('user');
    });

    it('requires a phone number (the schema exposes it as String!)', async () => {
      const res = await gql(ctx.app, {
        query: MUTATIONS.register,
        variables: {
          input: {
            email: 'nophone@example.com',
            password: 'Passw0rd1',
            firstName: 'John',
            lastName: 'Doe',
          },
        },
      });

      const err = res.body.errors[0];
      expect(err.extensions.code).toBe('BAD_USER_INPUT');
      expect(err.message).toContain('phoneNumber');
      expect(res.body.data ?? null).toBeNull();
      expect(await users().count()).toBe(0);
    });

    it('rejects a duplicated e-mail', async () => {
      await registerUser(ctx, { email: 'dup@example.com' });
      const second = await registerUser(ctx, {
        email: 'DUP@example.com',
        phoneNumber: nextPhone(),
      });

      expect(second.res.body.data ?? null).toBeNull();
      const err = second.res.body.errors[0];
      expect(err.message).toBe(en.EMAIL_EXISTED);
      expect(err.extensions.statusCode).toBe(400);
      expect(err.extensions.success).toBe(false);
      expect(await users().count()).toBe(1);
    });

    it('rejects a duplicated phone number', async () => {
      const phone = nextPhone();
      await registerUser(ctx, { phoneNumber: phone });
      const second = await registerUser(ctx, { phoneNumber: phone });

      expect(second.res.body.data ?? null).toBeNull();
      expect(second.res.body.errors[0].message).toBe(en.PHONE_EXISTED);
      expect(await users().count()).toBe(1);
    });

    it('rejects an already-used e-mail even when the payload differs', async () => {
      await registerUser(ctx, { email: 'taken@example.com' });
      const second = await registerUser(ctx, {
        email: 'taken@example.com',
        firstName: 'Other',
        phoneNumber: nextPhone(),
      });

      expect(second.res.body.errors[0].message).toBe(en.EMAIL_EXISTED);
      expect(ctx.kafka.emit).toHaveBeenCalledTimes(1);
      expect(ctx.outbox.enqueueEvent).toHaveBeenCalledTimes(1);
    });
  });

  describe('register input validation', () => {
    const registerWith = (input: Record<string, unknown>) =>
      gql(ctx.app, { query: MUTATIONS.register, variables: { input } });

    it('rejects a malformed e-mail address', async () => {
      const res = await registerWith({
        email: 'not-an-email',
        password: 'Passw0rd1',
        firstName: 'John',
        lastName: 'Doe',
        phoneNumber: nextPhone(),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe('Bad Request');
      expect(err.extensions.statusCode).toBe(400);
      expect(err.extensions.code).toBe('BAD_REQUEST');
      expect(err.extensions.success).toBe(false);
      expect(await users().count()).toBe(0);
    });

    it('rejects a password that is too short', async () => {
      const res = await registerWith({
        email: 'weak@example.com',
        password: 'Ab1',
        firstName: 'John',
        lastName: 'Doe',
        phoneNumber: nextPhone(),
      });

      expect(res.body.errors[0].message).toBe('Bad Request');
      expect(res.body.errors[0].extensions.statusCode).toBe(400);
      expect(await users().count()).toBe(0);
    });

    it('rejects a password without upper/lower/digit complexity', async () => {
      const res = await registerWith({
        email: 'weak2@example.com',
        password: 'abcdefghij',
        firstName: 'John',
        lastName: 'Doe',
        phoneNumber: nextPhone(),
      });

      expect(res.body.errors[0].message).toBe('Bad Request');
      expect(await users().count()).toBe(0);
    });

    it('rejects a phone number that is not a valid Egyptian number', async () => {
      const res = await registerWith({
        email: 'badphone@example.com',
        password: 'Passw0rd1',
        firstName: 'John',
        lastName: 'Doe',
        phoneNumber: '12345',
      });

      expect(res.body.errors[0].message).toBe('Bad Request');
      expect(await users().count()).toBe(0);
    });

    it('rejects a first name with digits', async () => {
      const res = await registerWith({
        email: 'badname@example.com',
        password: 'Passw0rd1',
        firstName: 'J0hn',
        lastName: 'Doe',
        phoneNumber: nextPhone(),
      });

      expect(res.body.errors[0].message).toBe('Bad Request');
      expect(await users().count()).toBe(0);
    });

    it('rejects unknown input fields (the schema wins over whitelist mode)', async () => {
      const res = await registerWith({
        email: 'extra@example.com',
        password: 'Passw0rd1',
        firstName: 'John',
        lastName: 'Doe',
        phoneNumber: nextPhone(),
        role: 'admin',
        isAdmin: true,
      });

      const err = res.body.errors[0];
      expect(err.extensions.code).toBe('BAD_USER_INPUT');
      expect(err.message).toContain('role');
      expect(res.body.data ?? null).toBeNull();
      expect(await users().count()).toBe(0);
    });
  });

  describe('register rate limit (3 per minute per IP)', () => {
    it('allows three attempts and answers 429 on the fourth', async () => {
      for (let i = 0; i < 3; i++) {
        const ok = await registerUser(ctx);
        expect(ok.res.body.errors).toBeUndefined();
      }

      const blocked = await registerUser(ctx);
      expect(blocked.res.body.data ?? null).toBeNull();

      const err = blocked.res.body.errors[0];
      expect(err.message).toBe('Too many requests, please try again later.');
      expect(err.extensions.statusCode).toBe(429);

      expect(blocked.res.headers['x-ratelimit-limit']).toBe('3');
      expect(blocked.res.headers['x-ratelimit-remaining']).toBe('0');
      expect(Number(blocked.res.headers['retry-after'])).toBeGreaterThan(0);

      expect(await users().count()).toBe(3);
    });
  });

  describe('login', () => {
    it('returns tokens and stores the session', async () => {
      await seedUser(ctx.dataSource, {
        email: 'LoginMe@example.com',
        firstName: 'Log',
      });

      const { res, body } = await loginUser(ctx, 'LOGINME@example.com');

      expect(res.body.errors).toBeUndefined();
      expect(body.success).toBe(true);
      expect(body.statusCode).toBe(200);
      expect(body.message).toBe(en.LOGIN_SUCCESS);
      expect(body.data.user.email).toBe('loginme@example.com');
      expect(body.data.user.firstName).toBe('Log');
      expect(typeof body.data.accessToken).toBe('string');
      expect(typeof body.data.refreshToken).toBe('string');
      expect(await ctx.redis.keys('user:session:*')).toHaveLength(1);
    });

    it('rejects an unknown e-mail with 400', async () => {
      const { res } = await loginUser(ctx, 'ghost@example.com');

      const err = res.body.errors[0];
      expect(err.message).toBe(en.INVALID_CREDENTIALS);
      expect(err.extensions.statusCode).toBe(400);
      expect(res.body.data ?? null).toBeNull();
    });

    it('rejects a wrong password with 401', async () => {
      await seedUser(ctx.dataSource, { email: 'wrongpw@example.com' });
      const { res } = await loginUser(
        ctx,
        'wrongpw@example.com',
        'NotThePass1',
      );

      const err = res.body.errors[0];
      expect(err.message).toBe(en.INVALID_CREDENTIALS);
      expect(err.extensions.statusCode).toBe(401);
      expect(await ctx.redis.keys('user:session:*')).toHaveLength(0);
    });

    it('does not log in a user twice for the same password typo', async () => {
      await seedUser(ctx.dataSource, { email: 'twice@example.com' });
      await loginUser(ctx, 'twice@example.com', 'Nope12345');
      await loginUser(ctx, 'twice@example.com', 'Nope12346');

      expect(await ctx.redis.keys('user:session:*')).toHaveLength(0);
    });
  });

  describe('login rate limit (5 per minute per IP)', () => {
    it('allows five attempts and answers 429 on the sixth', async () => {
      await seedUser(ctx.dataSource, { email: 'flood@example.com' });

      for (let i = 0; i < 5; i++) {
        const ok = await loginUser(ctx, 'flood@example.com');
        expect(ok.res.body.errors).toBeUndefined();
      }

      const blocked = await loginUser(ctx, 'flood@example.com');
      const err = blocked.res.body.errors[0];
      expect(err.message).toBe('Too many requests, please try again later.');
      expect(err.extensions.statusCode).toBe(429);
      expect(blocked.res.headers['x-ratelimit-limit']).toBe('5');
      expect(blocked.res.headers['x-ratelimit-remaining']).toBe('0');
      expect(Number(blocked.res.headers['retry-after'])).toBeGreaterThan(0);
      expect(await ctx.redis.keys('user:session:*')).toHaveLength(5);
    });
  });

  describe('per-IP fixed-window counter', () => {
    it('is shared by every rate-limited mutation (one key per IP)', async () => {
      // register, login, forgetPassword and resetPassword all consume the
      // same counter because the guard extracts only the client IP.
      const first = await registerUser(ctx); // count 0 < 3
      expect(first.res.body.errors).toBeUndefined();

      const second = await registerUser(ctx); // count 1 < 3
      expect(second.res.body.errors).toBeUndefined();

      const forget = await gql(ctx.app, {
        query: MUTATIONS.forgetPassword,
        variables: { input: { email: 'anyone@example.com' } },
      });
      // count 2 < 3: allowed, even though it is the first forgetPassword
      expect(forget.body.errors).toBeUndefined();

      const blocked = await gql(ctx.app, {
        query: MUTATIONS.forgetPassword,
        variables: { input: { email: 'anyone@example.com' } },
      });
      // count 3 >= 3: blocked although this is only the second forgetPassword
      expect(blocked.body.errors[0].extensions.statusCode).toBe(429);
      expect(blocked.body.errors[0].message).toBe(
        'Too many requests, please try again later.',
      );
    });
  });

  describe('logout', () => {
    it('revokes the current session', async () => {
      const registered = await registerUser(ctx);
      const token = registered.body.data.accessToken;
      const refresh = registered.body.data.refreshToken;

      const res = await gql(ctx.app, {
        query: MUTATIONS.logout,
        headers: auth(token),
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.logout.success).toBe(true);
      expect(res.body.data.logout.statusCode).toBe(200);
      expect(res.body.data.logout.message).toBe(en.LOGOUT_SUCCESS);
      expect(res.body.data.logout.data).toBe(true);
      expect(await ctx.redis.keys('user:session:*')).toHaveLength(0);

      const refreshed = await gql(ctx.app, {
        query: MUTATIONS.refreshToken,
        variables: { input: { refreshToken: refresh } },
      });
      expect(refreshed.body.errors[0].message).toBe(en.SESSION_EXPIRED);
    });

    it('requires an access token', async () => {
      const res = await gql(ctx.app, { query: MUTATIONS.logout });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.NO_TOKEN);
      expect(err.extensions.statusCode).toBe(401);
    });

    it('rejects a tampered access token', async () => {
      const res = await gql(ctx.app, {
        query: MUTATIONS.logout,
        headers: auth('not.a.jwt'),
      });

      expect(res.body.errors[0].message).toBe(en.INVALID_TOKEN);
      expect(res.body.errors[0].extensions.statusCode).toBe(401);
    });
  });

  describe('refreshToken', () => {
    it('issues a new pair of usable tokens', async () => {
      const registered = await registerUser(ctx);
      const oldAccess = registered.body.data.accessToken;

      // JWTs embed `iat` in seconds; wait for the next second so the newly
      // signed pair is guaranteed to differ from the one under test.
      await new Promise((resolve) => setTimeout(resolve, 1100));

      const res = await gql(ctx.app, {
        query: MUTATIONS.refreshToken,
        variables: {
          input: { refreshToken: registered.body.data.refreshToken },
        },
      });

      expect(res.body.errors).toBeUndefined();
      const payload = res.body.data.refreshToken;
      expect(payload.success).toBe(true);
      expect(payload.message).toBe(en.REFRESH_TOKEN_SUCCESS);
      expect(payload.data.accessToken).not.toBe(oldAccess);

      const profile = await gql(ctx.app, {
        query: QUERIES.myProfile,
        headers: auth(payload.data.accessToken),
      });
      expect(profile.body.errors).toBeUndefined();
      expect(profile.body.data.myProfile.data.id).toBe(
        registered.body.data.user.id,
      );
    });

    it('rejects a garbage refresh token', async () => {
      const res = await gql(ctx.app, {
        query: MUTATIONS.refreshToken,
        variables: { input: { refreshToken: 'x'.repeat(40) } },
      });

      expect(res.body.errors[0].message).toBe(en.INVALID_TOKEN);
      expect(res.body.errors[0].extensions.statusCode).toBe(401);
    });

    it('rejects an access token used as a refresh token', async () => {
      const registered = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: MUTATIONS.refreshToken,
        variables: {
          input: { refreshToken: registered.body.data.accessToken },
        },
      });

      expect(res.body.errors[0].message).toBe(en.INVALID_TOKEN);
      expect(res.body.errors[0].extensions.statusCode).toBe(401);
    });

    it('rejects a refresh token whose session was revoked', async () => {
      const registered = await registerUser(ctx);
      await gql(ctx.app, {
        query: MUTATIONS.logout,
        headers: auth(registered.body.data.accessToken),
      });

      const res = await gql(ctx.app, {
        query: MUTATIONS.refreshToken,
        variables: {
          input: { refreshToken: registered.body.data.refreshToken },
        },
      });

      expect(res.body.errors[0].message).toBe(en.SESSION_EXPIRED);
    });

    it('rejects a refresh token when the account was deleted', async () => {
      const registered = await registerUser(ctx);
      await users().delete({ id: registered.body.data.user.id });

      const res = await gql(ctx.app, {
        query: MUTATIONS.refreshToken,
        variables: {
          input: { refreshToken: registered.body.data.refreshToken },
        },
      });

      expect(res.body.errors[0].message).toBe(en.NOT_FOUND);
      expect(res.body.errors[0].extensions.statusCode).toBe(401);
    });
  });

  describe('forgetPassword', () => {
    it('succeeds silently for an unknown e-mail (no notification)', async () => {
      const res = await gql(ctx.app, {
        query: MUTATIONS.forgetPassword,
        variables: { input: { email: 'ghost@example.com' } },
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.forgetPassword.success).toBe(true);
      expect(res.body.data.forgetPassword.statusCode).toBe(200);
      expect(res.body.data.forgetPassword.message).toBe(
        en.FORGET_PASSWORD_SUCCESS,
      );
      expect(res.body.data.forgetPassword.data).toBe(true);
      expect(ctx.notification.send).not.toHaveBeenCalled();
    });

    it('stores a six digit code with a 15 minute expiry and notifies by e-mail', async () => {
      await seedUser(ctx.dataSource, {
        email: 'resetme@example.com',
        firstName: 'Rita',
      });

      const res = await gql(ctx.app, {
        query: MUTATIONS.forgetPassword,
        variables: { input: { email: 'RESETME@example.com' } },
      });

      expect(res.body.errors).toBeUndefined();
      expect(ctx.notification.send).toHaveBeenCalledTimes(1);

      const [channel, message] = ctx.notification.send.mock.calls[0];
      expect(channel).toBe(ChannelType.EMAIL);
      expect(message.recipientId).toBe('resetme@example.com');
      expect(message.subject).toBe('Password Reset Code');
      expect(message.context.name).toBe('Rita');
      expect(String(message.context.token)).toMatch(/^\d{6}$/);

      const row = await users().findOne({
        where: { email: 'resetme@example.com' },
      });
      expect(row?.resetToken).toBe(String(message.context.token));
      const expiry = row?.resetTokenExpiry?.getTime() ?? 0;
      expect(expiry).toBeGreaterThan(Date.now());
      expect(expiry).toBeLessThanOrEqual(Date.now() + 16 * 60 * 1000);

      expect(ctx.kafka.emit).toHaveBeenCalledWith(
        UserKafkaTopics.PASSWORD_RESET_REQUESTED,
        UserKafkaTopics.PASSWORD_RESET_REQUESTED,
        expect.objectContaining({ email: 'resetme@example.com' }),
      );
    });

    it('is rate limited to three requests per minute', async () => {
      await seedUser(ctx.dataSource, { email: 'limited@example.com' });

      for (let i = 0; i < 3; i++) {
        const ok = await gql(ctx.app, {
          query: MUTATIONS.forgetPassword,
          variables: { input: { email: 'limited@example.com' } },
        });
        expect(ok.body.errors).toBeUndefined();
      }

      const blocked = await gql(ctx.app, {
        query: MUTATIONS.forgetPassword,
        variables: { input: { email: 'limited@example.com' } },
      });
      expect(blocked.body.errors[0].extensions.statusCode).toBe(429);
    });
  });

  describe('resetPassword', () => {
    const requestCode = async (email: string): Promise<string> => {
      await gql(ctx.app, {
        query: MUTATIONS.forgetPassword,
        variables: { input: { email } },
      });
      const row = await users().findOne({ where: { email } });
      return row?.resetToken as string;
    };

    it('rejects an unknown token', async () => {
      const res = await gql(ctx.app, {
        query: MUTATIONS.resetPassword,
        variables: { input: { token: '000000', passwordNew: 'BrandNew1' } },
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.INVALID_RESET_TOKEN);
      expect(err.extensions.statusCode).toBe(400);
    });

    it('rejects an expired token', async () => {
      const row = await seedUser(ctx.dataSource, {
        email: 'expired@example.com',
      });
      await users().update(row.id, {
        resetToken: '123456',
        resetTokenExpiry: new Date(Date.now() - 60 * 1000),
      });

      const res = await gql(ctx.app, {
        query: MUTATIONS.resetPassword,
        variables: { input: { token: '123456', passwordNew: 'BrandNew1' } },
      });

      expect(res.body.errors[0].message).toBe(en.INVALID_RESET_TOKEN);
    });

    it('rotates the password so the new one works and the old one does not', async () => {
      await seedUser(ctx.dataSource, { email: 'changeit@example.com' });
      const token = await requestCode('changeit@example.com');
      expect(token).toMatch(/^\d{6}$/);

      const res = await gql(ctx.app, {
        query: MUTATIONS.resetPassword,
        variables: { input: { token, passwordNew: 'BrandNew1' } },
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.resetPassword.success).toBe(true);
      expect(res.body.data.resetPassword.message).toBe(
        en.RESET_PASSWORD_SUCCESS,
      );
      expect(res.body.data.resetPassword.data).toBe(true);

      // new password works, old one does not
      const ok = await loginUser(ctx, 'changeit@example.com', 'BrandNew1');
      expect(ok.res.body.errors).toBeUndefined();

      const old = await loginUser(ctx, 'changeit@example.com', 'Passw0rd1');
      expect(old.res.body.errors[0].message).toBe(en.INVALID_CREDENTIALS);

      // Known behavior: `resetPassword` assigns `undefined` to the token
      // columns, which TypeORM skips on UPDATE, so the previous values are
      // still there until the 15-minute expiry closes the window.
      const row = await users().findOne({
        where: { email: 'changeit@example.com' },
      });
      expect(row?.resetToken).toBe(token);
      expect(row?.resetTokenExpiry?.getTime()).toBeGreaterThan(Date.now());
    });

    it('accepts a replay of the code while its window is still open', async () => {
      // Documents the same known behavior: because the columns are not
      // cleared, the code remains usable until it expires.
      await seedUser(ctx.dataSource, { email: 'replay@example.com' });
      const token = await requestCode('replay@example.com');

      const first = await gql(ctx.app, {
        query: MUTATIONS.resetPassword,
        variables: { input: { token, passwordNew: 'BrandNew1' } },
      });
      expect(first.body.errors).toBeUndefined();

      const replay = await gql(ctx.app, {
        query: MUTATIONS.resetPassword,
        variables: { input: { token, passwordNew: 'Another1' } },
      });
      expect(replay.body.errors).toBeUndefined();
      expect(replay.body.data.resetPassword.data).toBe(true);

      // the replayed password is the one that is now active
      const login = await loginUser(ctx, 'replay@example.com', 'Another1');
      expect(login.res.body.errors).toBeUndefined();
    });
  });
});
