import { Role, UserKafkaTopics } from '@delivery-micro/shard';
import { User } from '../src/common/database/entities/user.entity';
import { Address } from '../src/common/database/entities/address.entity';
import { Outbox } from '../src/common/database/entities/outbox.entity';
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
} from './support/harness';

const en = loadLocale('en');

/**
 * Profile reads and writes, the address book, the admin operations and the
 * federation surface (`_service` / `_entities`), including the permission
 * checks the RoleGuard enforces and every side effect the mutations cause
 * (Postgres rows, the read-through Redis cache, the transactional outbox,
 * the Kafka events and the RabbitMQ broadcasts).
 */
describe('Profile, addresses, admin operations and federation', () => {
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
  const addresses = () => ctx.dataSource.getRepository(Address);
  const outbox = () => ctx.dataSource.getRepository(Outbox);

  const UNKNOWN_ID = '00000000-0000-4000-8000-000000000000';

  describe('profile queries', () => {
    it('returns the signed-in account for myProfile', async () => {
      const registered = await registerUser(ctx, {
        firstName: 'Alice',
        lastName: 'Smith',
      });
      const token = registered.body.data.accessToken;
      const id = registered.body.data.user.id;

      const res = await gql(ctx.app, {
        query: QUERIES.myProfile,
        headers: auth(token),
      });

      expect(res.status).toBe(200);
      expect(res.body.errors).toBeUndefined();

      const profile = res.body.data.myProfile;
      expect(profile.success).toBe(true);
      expect(profile.statusCode).toBe(200);
      expect(profile.message).toBe(en.PROFILE_RETRIEVED);
      expect(profile.data.id).toBe(id);
      expect(profile.data.email).toBe(registered.input.email);
      expect(profile.data.firstName).toBe('Alice');
      expect(profile.data.lastName).toBe('Smith');
      expect(profile.data.role).toBe('admin');
      expect(profile.data.isActive).toBe(true);
      expect(profile.data.addresses).toEqual([]);
      expect(typeof profile.data.createdAt).toBe('string');

      // the read-through cache now holds the document
      expect(await ctx.redis.keys(`user:${id}`)).toHaveLength(1);
    });

    it('rejects myProfile without a token', async () => {
      const res = await gql(ctx.app, { query: QUERIES.myProfile });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.NO_TOKEN);
      expect(err.extensions.statusCode).toBe(401);
      expect(res.body.data ?? null).toBeNull();
    });

    it('lets an admin read another account through user(id)', async () => {
      const admin = await registerUser(ctx);
      const target = await seedUser(ctx.dataSource, {
        email: 'target@example.com',
        firstName: 'Tara',
        role: Role.USER,
      });

      const res = await gql(ctx.app, {
        query: QUERIES.user,
        variables: { id: target.id },
        headers: auth(admin.body.data.accessToken),
      });

      expect(res.body.errors).toBeUndefined();

      const payload = res.body.data.user;
      expect(payload.success).toBe(true);
      expect(payload.statusCode).toBe(200);
      expect(payload.message).toBe(en.USER_RETRIEVED);
      expect(payload.data.id).toBe(target.id);
      expect(payload.data.email).toBe('target@example.com');
      expect(payload.data.firstName).toBe('Tara');
      expect(payload.data.role).toBe('user');
      expect(payload.data.addresses).toEqual([]);
    });

    it('hides user(id) from accounts without the VIEW_USER permission', async () => {
      await seedUser(ctx.dataSource, {
        email: 'seed-admin@example.com',
        role: Role.ADMIN,
      });
      const plain = await registerUser(ctx);
      expect(plain.body.data.user.role).toBe('user');

      const res = await gql(ctx.app, {
        query: QUERIES.user,
        variables: { id: plain.body.data.user.id },
        headers: auth(plain.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.INSUFFICIENT_PERMISSIONS);
      expect(err.extensions.statusCode).toBe(401);
      expect(res.body.data ?? null).toBeNull();
    });

    it('answers with null data for an unknown user id', async () => {
      const admin = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: QUERIES.user,
        variables: { id: UNKNOWN_ID },
        headers: auth(admin.body.data.accessToken),
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.user.success).toBe(true);
      expect(res.body.data.user.data).toBeNull();
    });
  });

  describe('updateProfile', () => {
    it('renames the profile and refreshes every side channel', async () => {
      const registered = await registerUser(ctx, { firstName: 'Old' });
      const token = registered.body.data.accessToken;
      const id = registered.body.data.user.id;

      const res = await gql(ctx.app, {
        query: MUTATIONS.updateProfile,
        variables: { input: { firstName: 'Renamed', lastName: 'Profile' } },
        headers: auth(token),
      });

      expect(res.body.errors).toBeUndefined();

      // TextField() normalises every word to lower case before persisting.
      const payload = res.body.data.updateProfile;
      expect(payload.success).toBe(true);
      expect(payload.statusCode).toBe(200);
      expect(payload.message).toBe(en.PROFILE_UPDATED);
      expect(payload.data.firstName).toBe('renamed');
      expect(payload.data.lastName).toBe('profile');

      const row = await users().findOne({ where: { id } });
      expect(row?.firstName).toBe('renamed');
      expect(row?.lastName).toBe('profile');

      // transactional outbox row + immediate BullMQ dispatch
      const events = await outbox().find();
      const updates = events.filter((e) => e.eventType === 'user.updated');
      expect(updates).toHaveLength(1);
      expect(updates[0].aggregateType).toBe('User');
      expect(updates[0].processed).toBe(false);
      expect(updates[0].payload).toMatchObject({
        userId: id,
        firstName: 'renamed',
        lastName: 'profile',
      });
      expect(ctx.outbox.enqueueEvent).toHaveBeenCalledWith(updates[0].id);

      // search + RabbitMQ fan-out
      expect(ctx.kafka.emit).toHaveBeenCalledWith(
        UserKafkaTopics.USER_UPDATED,
        UserKafkaTopics.USER_UPDATED,
        expect.objectContaining({ userId: id, firstName: 'renamed' }),
      );
      expect(ctx.rabbit.publishUserUpdated).toHaveBeenCalledWith(
        expect.objectContaining({ id, email: registered.input.email }),
      );

      // the read-through cache holds the new document
      const cached = await ctx.redis.hGet<string>(`user:${id}`, 'data');
      expect(cached).toContain('renamed');
    });

    it('resolves an avatar through the media service', async () => {
      const registered = await registerUser(ctx);
      const token = registered.body.data.accessToken;

      const res = await gql(ctx.app, {
        query: MUTATIONS.updateProfile,
        variables: { input: { avatarMediaId: 'avatar-99' } },
        headers: auth(token),
      });

      expect(res.body.errors).toBeUndefined();
      expect(ctx.media.resolveMediaUrl).toHaveBeenCalledWith(
        expect.objectContaining({
          mediaId: 'avatar-99',
          requesterId: registered.body.data.user.id,
        }),
      );
      expect(res.body.data.updateProfile.data.imageUrl).toBe(
        'https://media.test/avatars/resolved.png',
      );
    });

    it('rejects an image url that is not http(s)', async () => {
      const registered = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: MUTATIONS.updateProfile,
        variables: { input: { imageUrl: 'ftp://files.example.com/a.png' } },
        headers: auth(registered.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.INVALID_IMAGE_URL);
      expect(err.extensions.statusCode).toBe(400);
      expect(res.body.data ?? null).toBeNull();

      const row = await users().findOne({
        where: { id: registered.body.data.user.id },
      });
      expect(row?.imageUrl).toBeNull();
    });

    it('rejects a first name below the minimum length', async () => {
      const registered = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: MUTATIONS.updateProfile,
        variables: { input: { firstName: 'A' } },
        headers: auth(registered.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.extensions.statusCode).toBe(400);
      expect(res.body.data ?? null).toBeNull();

      const row = await users().findOne({
        where: { id: registered.body.data.user.id },
      });
      expect(row?.firstName).toBe('John');
    });
  });

  describe('changePassword', () => {
    it('rotates the password so only the new one works', async () => {
      const registered = await registerUser(ctx);
      const token = registered.body.data.accessToken;

      const res = await gql(ctx.app, {
        query: MUTATIONS.changePassword,
        variables: {
          input: { passwordOld: 'Passw0rd1', passwordNew: 'BrandNew1' },
        },
        headers: auth(token),
      });

      expect(res.body.errors).toBeUndefined();

      const payload = res.body.data.changePassword;
      expect(payload.success).toBe(true);
      expect(payload.statusCode).toBe(200);
      expect(payload.message).toBe(en.PASSWORD_CHANGED);
      expect(payload.data).toBe(true);

      const fresh = await loginUser(ctx, registered.input.email, 'BrandNew1');
      expect(fresh.res.body.errors).toBeUndefined();
      expect(fresh.body.data.user.id).toBe(registered.body.data.user.id);

      const stale = await loginUser(ctx, registered.input.email, 'Passw0rd1');
      expect(stale.res.body.errors[0].message).toBe(en.INVALID_CREDENTIALS);
      expect(stale.res.body.errors[0].extensions.statusCode).toBe(401);
    });

    it('rejects a wrong current password', async () => {
      const registered = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: MUTATIONS.changePassword,
        variables: {
          input: { passwordOld: 'NotThePass1', passwordNew: 'BrandNew1' },
        },
        headers: auth(registered.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.INVALID_PASSWORD);
      expect(err.extensions.statusCode).toBe(401);
      expect(res.body.data ?? null).toBeNull();
    });
  });

  describe('addresses', () => {
    const newAddress = (overrides: Record<string, unknown> = {}) => ({
      title: 'Home',
      street: '123 Main St',
      city: 'Cairo',
      state: 'CA',
      postalCode: '11511',
      latitude: 30.04,
      longitude: 31.23,
      ...overrides,
    });

    const addAddress = (
      token: string,
      overrides: Record<string, unknown> = {},
    ) =>
      gql(ctx.app, {
        query: MUTATIONS.addAddress,
        variables: { input: newAddress(overrides) },
        headers: auth(token),
      });

    it('adds an address and lists it everywhere', async () => {
      const registered = await registerUser(ctx);
      const token = registered.body.data.accessToken;
      const id = registered.body.data.user.id;

      const res = await addAddress(token);

      expect(res.body.errors).toBeUndefined();

      const payload = res.body.data.addAddress;
      expect(payload.success).toBe(true);
      expect(payload.statusCode).toBe(200);
      expect(payload.message).toBe(en.ADDRESS_ADDED);
      expect(payload.data.userId).toBe(id);
      // TextField() normalises the address fields to lower case as well.
      expect(payload.data.title).toBe('home');
      expect(payload.data.street).toBe('123 main st');
      expect(payload.data.city).toBe('cairo');
      expect(payload.data.state).toBe('ca');
      expect(payload.data.postalCode).toBe('11511');
      expect(payload.data.latitude).toBeCloseTo(30.04);
      expect(payload.data.longitude).toBeCloseTo(31.23);
      expect(payload.data.isDefault).toBe(false);
      expect(typeof payload.data.id).toBe('string');
      expect(typeof payload.data.createdAt).toBe('string');

      const list = await gql(ctx.app, {
        query: QUERIES.myAddresses,
        headers: auth(token),
      });
      expect(list.body.errors).toBeUndefined();
      expect(list.body.data.myAddresses.data).toHaveLength(1);
      expect(list.body.data.myAddresses.data[0].id).toBe(payload.data.id);

      const profile = await gql(ctx.app, {
        query: QUERIES.myProfile,
        headers: auth(token),
      });
      expect(profile.body.data.myProfile.data.addresses).toHaveLength(1);
      expect(profile.body.data.myProfile.data.addresses[0].id).toBe(
        payload.data.id,
      );
    });

    it('rejects an address without a street (schema non-null)', async () => {
      const registered = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: MUTATIONS.addAddress,
        variables: { input: { title: 'Home', city: 'Cairo' } },
        headers: auth(registered.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.extensions.code).toBe('BAD_USER_INPUT');
      expect(err.message).toContain('street');
      expect(res.body.data ?? null).toBeNull();
      expect(await addresses().count()).toBe(0);
    });

    it('keeps exactly one default address', async () => {
      const registered = await registerUser(ctx);
      const token = registered.body.data.accessToken;

      const first = await addAddress(token, { isDefault: true });
      const second = await addAddress(token, {
        title: 'Work',
        street: '2 Work St',
        city: 'Giza',
        isDefault: true,
      });

      expect(first.body.errors).toBeUndefined();
      expect(second.body.errors).toBeUndefined();

      const homeId = first.body.data.addAddress.data.id;
      const workId = second.body.data.addAddress.data.id;
      expect(first.body.data.addAddress.data.isDefault).toBe(true);
      expect(second.body.data.addAddress.data.isDefault).toBe(true);

      // adding a new default demoted the previous one
      const afterAdd = await addresses().find();
      expect(afterAdd.find((a) => a.id === homeId)?.isDefault).toBe(false);
      expect(afterAdd.find((a) => a.id === workId)?.isDefault).toBe(true);

      const res = await gql(ctx.app, {
        query: MUTATIONS.setDefaultAddress,
        variables: { addressId: homeId },
        headers: auth(token),
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.setDefaultAddress.success).toBe(true);
      expect(res.body.data.setDefaultAddress.message).toBe(
        en.DEFAULT_ADDRESS_UPDATED,
      );
      expect(res.body.data.setDefaultAddress.data).toBe(true);

      const afterSet = await addresses().find();
      expect(afterSet.find((a) => a.id === homeId)?.isDefault).toBe(true);
      expect(afterSet.find((a) => a.id === workId)?.isDefault).toBe(false);

      // the cached profile reflects the new default
      const profile = await gql(ctx.app, {
        query: QUERIES.myProfile,
        headers: auth(token),
      });
      const home = profile.body.data.myProfile.data.addresses.find(
        (a: { id: string }) => a.id === homeId,
      );
      expect(home.isDefault).toBe(true);
    });

    it('deletes an address', async () => {
      const registered = await registerUser(ctx);
      const token = registered.body.data.accessToken;

      const added = await addAddress(token);
      const addressId = added.body.data.addAddress.data.id;

      const res = await gql(ctx.app, {
        query: MUTATIONS.deleteAddress,
        variables: { addressId },
        headers: auth(token),
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deleteAddress.success).toBe(true);
      expect(res.body.data.deleteAddress.message).toBe(en.ADDRESS_DELETED);
      expect(res.body.data.deleteAddress.data).toBe(true);

      expect(await addresses().count()).toBe(0);

      const list = await gql(ctx.app, {
        query: QUERIES.myAddresses,
        headers: auth(token),
      });
      expect(list.body.data.myAddresses.data).toEqual([]);
    });

    it('refuses to delete an address that belongs to someone else', async () => {
      const owner = await registerUser(ctx);
      const added = await addAddress(owner.body.data.accessToken);
      const addressId = added.body.data.addAddress.data.id;

      await seedUser(ctx.dataSource, { email: 'intruder@example.com' });
      const intruder = await loginUser(ctx, 'intruder@example.com');

      const res = await gql(ctx.app, {
        query: MUTATIONS.deleteAddress,
        variables: { addressId },
        headers: auth(intruder.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.ADDRESS_NOT_FOUND);
      expect(err.extensions.statusCode).toBe(400);
      expect(await addresses().count()).toBe(1);
    });

    it('refuses to promote a foreign address to default', async () => {
      const owner = await registerUser(ctx);
      const added = await addAddress(owner.body.data.accessToken, {
        isDefault: true,
      });
      const addressId = added.body.data.addAddress.data.id;

      await seedUser(ctx.dataSource, { email: 'other@example.com' });
      const other = await loginUser(ctx, 'other@example.com');

      const res = await gql(ctx.app, {
        query: MUTATIONS.setDefaultAddress,
        variables: { addressId },
        headers: auth(other.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.ADDRESS_NOT_FOUND);
      expect(err.extensions.statusCode).toBe(400);

      const row = await addresses().findOne({ where: { id: addressId } });
      expect(row?.isDefault).toBe(true);
    });
  });

  describe('admin operations', () => {
    it('promotes a user to admin', async () => {
      const admin = await registerUser(ctx);
      const target = await seedUser(ctx.dataSource, {
        email: 'promoteme@example.com',
        role: Role.USER,
      });

      const res = await gql(ctx.app, {
        query: MUTATIONS.promoteUserToAdmin,
        variables: { id: target.id },
        headers: auth(admin.body.data.accessToken),
      });

      expect(res.body.errors).toBeUndefined();

      const payload = res.body.data.promoteUserToAdmin;
      expect(payload.success).toBe(true);
      expect(payload.statusCode).toBe(200);
      expect(payload.message).toBe(en.USER_PROMOTED);
      expect(payload.data).toBe(true);

      const row = await users().findOne({ where: { id: target.id } });
      expect(row?.role).toBe(Role.ADMIN);

      expect(ctx.kafka.emit).toHaveBeenCalledWith(
        UserKafkaTopics.USER_UPDATED,
        UserKafkaTopics.USER_UPDATED,
        expect.objectContaining({ userId: target.id, role: Role.ADMIN }),
      );
      expect(ctx.rabbit.publishUserUpdated).toHaveBeenCalledWith(
        expect.objectContaining({ id: target.id }),
      );
    });

    it('rejects promoting an unknown user', async () => {
      const admin = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: MUTATIONS.promoteUserToAdmin,
        variables: { id: UNKNOWN_ID },
        headers: auth(admin.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.NOT_FOUND);
      expect(err.extensions.statusCode).toBe(404);
      expect(res.body.data ?? null).toBeNull();
    });

    it('toggles the active flag off and back on', async () => {
      const admin = await registerUser(ctx);
      const target = await seedUser(ctx.dataSource, {
        email: 'toggleme@example.com',
      });

      const off = await gql(ctx.app, {
        query: MUTATIONS.toggleUserActive,
        variables: { id: target.id, isActive: false },
        headers: auth(admin.body.data.accessToken),
      });

      expect(off.body.errors).toBeUndefined();
      expect(off.body.data.toggleUserActive.message).toBe(
        en.USER_STATUS_UPDATED,
      );
      expect(off.body.data.toggleUserActive.data).toBe(true);
      expect(
        (await users().findOne({ where: { id: target.id } }))?.isActive,
      ).toBe(false);

      const on = await gql(ctx.app, {
        query: MUTATIONS.toggleUserActive,
        variables: { id: target.id, isActive: true },
        headers: auth(admin.body.data.accessToken),
      });

      expect(on.body.errors).toBeUndefined();
      expect(
        (await users().findOne({ where: { id: target.id } }))?.isActive,
      ).toBe(true);
      expect(ctx.rabbit.publishUserUpdated).toHaveBeenCalledWith(
        expect.objectContaining({ id: target.id, isActive: true }),
      );
    });

    it('rejects toggling an unknown user', async () => {
      const admin = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: MUTATIONS.toggleUserActive,
        variables: { id: UNKNOWN_ID, isActive: false },
        headers: auth(admin.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.NOT_FOUND);
      expect(err.extensions.statusCode).toBe(404);
    });

    it('deletes a user and clears every trace', async () => {
      const admin = await registerUser(ctx);
      const target = await seedUser(ctx.dataSource, {
        email: 'deleteme@example.com',
      });

      // warm the read-through cache first
      await gql(ctx.app, {
        query: QUERIES.user,
        variables: { id: target.id },
        headers: auth(admin.body.data.accessToken),
      });
      expect(await ctx.redis.keys(`user:${target.id}`)).toHaveLength(1);

      const res = await gql(ctx.app, {
        query: MUTATIONS.deleteUser,
        variables: { id: target.id },
        headers: auth(admin.body.data.accessToken),
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data.deleteUser.success).toBe(true);
      expect(res.body.data.deleteUser.message).toBe(en.USER_DELETED);
      expect(res.body.data.deleteUser.data).toBe(true);

      expect(await users().findOne({ where: { id: target.id } })).toBeNull();

      expect(ctx.kafka.emit).toHaveBeenCalledWith(
        UserKafkaTopics.USER_DELETED,
        UserKafkaTopics.USER_DELETED,
        expect.objectContaining({ userId: target.id }),
      );
      expect(ctx.rabbit.publishUserDeleted).toHaveBeenCalledWith(target.id);

      // the cached document is gone as well
      expect(await ctx.redis.keys(`user:${target.id}`)).toHaveLength(0);
    });

    it('rejects deleting an unknown user', async () => {
      const admin = await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: MUTATIONS.deleteUser,
        variables: { id: UNKNOWN_ID },
        headers: auth(admin.body.data.accessToken),
      });

      const err = res.body.errors[0];
      expect(err.message).toBe(en.NOT_FOUND);
      expect(err.extensions.statusCode).toBe(404);
    });

    it('refuses the admin mutations to a plain user', async () => {
      await seedUser(ctx.dataSource, {
        email: 'root@example.com',
        role: Role.ADMIN,
      });
      const plain = await registerUser(ctx);
      expect(plain.body.data.user.role).toBe('user');

      const token = plain.body.data.accessToken;
      const targetId = plain.body.data.user.id;

      const promote = await gql(ctx.app, {
        query: MUTATIONS.promoteUserToAdmin,
        variables: { id: targetId },
        headers: auth(token),
      });
      expect(promote.body.errors[0].message).toBe(en.INSUFFICIENT_PERMISSIONS);
      expect(promote.body.errors[0].extensions.statusCode).toBe(401);

      const remove = await gql(ctx.app, {
        query: MUTATIONS.deleteUser,
        variables: { id: targetId },
        headers: auth(token),
      });
      expect(remove.body.errors[0].message).toBe(en.INSUFFICIENT_PERMISSIONS);

      const toggle = await gql(ctx.app, {
        query: MUTATIONS.toggleUserActive,
        variables: { id: targetId, isActive: false },
        headers: auth(token),
      });
      expect(toggle.body.errors[0].message).toBe(en.INSUFFICIENT_PERMISSIONS);

      // the account is untouched
      const row = await users().findOne({ where: { id: targetId } });
      expect(row?.role).toBe(Role.USER);
      expect(row?.isActive).toBe(true);
    });
  });

  describe('federation', () => {
    it('exposes the User key through the federation sdl', async () => {
      const res = await gql(ctx.app, { query: QUERIES.service });

      expect(res.body.errors).toBeUndefined();
      const sdl = res.body.data._service.sdl;
      expect(typeof sdl).toBe('string');
      expect(sdl).toContain('type User');
      expect(sdl).toContain('@key(fields: "id")');
      expect(sdl).toContain('type AddressType');
    });

    it('resolves a User representation through _entities', async () => {
      const registered = await registerUser(ctx, { firstName: 'Feder' });
      const id = registered.body.data.user.id;

      const res = await gql(ctx.app, {
        query: QUERIES.entities,
        variables: { representations: [{ __typename: 'User', id }] },
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data._entities).toEqual([
        {
          id,
          email: registered.input.email,
          firstName: 'Feder',
          lastName: 'Doe',
          role: 'admin',
        },
      ]);
    });

    it('returns null for an unknown representation', async () => {
      await registerUser(ctx);

      const res = await gql(ctx.app, {
        query: QUERIES.entities,
        variables: {
          representations: [{ __typename: 'User', id: UNKNOWN_ID }],
        },
      });

      expect(res.body.errors).toBeUndefined();
      expect(res.body.data._entities).toEqual([null]);
    });
  });
});
