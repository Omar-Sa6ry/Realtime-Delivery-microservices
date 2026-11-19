import { DataSource } from 'typeorm';
import { User } from './user.entity';
import { Address } from './address.entity';
import { Outbox } from './outbox.entity';
import { Role } from '@delivery-micro/shard';

/**
 * The relation decorators hold lazy thunks (`() => Address`) that TypeORM only
 * resolves while it builds the entity metadata. Building the metadata against
 * a dormant postgres data source exercises that code path without opening a
 * real connection (the dialect must be postgres so that `timestamptz` and
 * `jsonb` columns validate).
 */
describe('database entities', () => {
  let dataSource: DataSource;
  let userMeta: any;
  let addressMeta: any;
  let outboxMeta: any;

  beforeAll(async () => {
    dataSource = new DataSource({
      type: 'postgres',
      host: 'localhost',
      port: 5433,
      username: 'unused',
      password: 'unused',
      database: 'unused',
      entities: [User, Address, Outbox],
      synchronize: false,
    });

    // `buildMetadatas` is protected on `DataSource`; the test only needs the
    // entity metadata registry that it populates.
    await (
      dataSource as unknown as { buildMetadatas: () => Promise<void> }
    ).buildMetadatas();

    const metadatas = dataSource.entityMetadatas;
    userMeta = metadatas.find((m) => m.name === 'User');
    addressMeta = metadatas.find((m) => m.name === 'Address');
    outboxMeta = metadatas.find((m) => m.name === 'Outbox');
  });

  afterAll(async () => {
    if (dataSource?.isInitialized) {
      await dataSource.destroy();
    }
  });

  describe('User', () => {
    it('maps to the users table', () => {
      expect(userMeta.tableName).toBe('users');
    });

    it('uses the varchar surrogate primary key', () => {
      expect(userMeta.primaryColumns).toHaveLength(1);
      expect(userMeta.primaryColumns[0].databaseName).toBe('id');
      expect(userMeta.primaryColumns[0].type).toBe('varchar');
    });

    it('maps the credential and profile columns', () => {
      const columns = Object.fromEntries(
        userMeta.columns.map((c: any) => [c.propertyName, c.databaseName]),
      );

      expect(columns).toMatchObject({
        id: 'id',
        email: 'email',
        passwordHash: 'password_hash',
        firstName: 'first_name',
        lastName: 'last_name',
        phoneNumber: 'phone_number',
        role: 'role',
        isActive: 'is_active',
        resetToken: 'reset_token',
        resetTokenExpiry: 'reset_token_expiry',
        imageUrl: 'image_url',
        createdAt: 'created_at',
        updatedAt: 'updated_at',
      });
    });

    it('enforces uniqueness on the e-mail and phone number', () => {
      const unique = userMeta.uniques.map((u: any) =>
        u.columns.map((c: any) => c.propertyName),
      );

      expect(unique).toEqual(
        expect.arrayContaining([['email'], ['phoneNumber']]),
      );
    });

    it('defaults the role to the shared user role', () => {
      const role = userMeta.columns.find((c: any) => c.propertyName === 'role');

      expect(role.default).toBe(Role.USER);
    });

    it('defaults the account to active', () => {
      const isActive = userMeta.columns.find(
        (c: any) => c.propertyName === 'isActive',
      );

      expect(isActive.default).toBe(true);
    });

    it('allows the reset token columns to be absent', () => {
      const nullable = userMeta.columns
        .filter((c: any) => c.isNullable)
        .map((c: any) => c.propertyName);

      expect(nullable).toEqual(
        expect.arrayContaining(['resetToken', 'resetTokenExpiry', 'imageUrl']),
      );
    });

    it('stores the reset token expiry as a timestamptz column', () => {
      const expiry = userMeta.columns.find(
        (c: any) => c.propertyName === 'resetTokenExpiry',
      );

      expect(expiry.type).toBe('timestamptz');
    });

    it('relates to the addresses eagerly and with cascade', () => {
      const relation = userMeta.relations.find(
        (r: any) => r.propertyName === 'addresses',
      );

      expect(relation).toBeDefined();
      expect(relation.relationType).toBe('one-to-many');
      expect(relation.inverseEntityMetadata.name).toBe('Address');
      expect(relation.isEager).toBe(true);
      expect(relation.isCascadeInsert).toBe(true);
      expect(relation.isCascadeUpdate).toBe(true);
      expect(relation.isCascadeRemove).toBe(true);
    });
  });

  describe('Address', () => {
    it('maps to the addresses table', () => {
      expect(addressMeta.tableName).toBe('addresses');
    });

    it('maps the location and geo columns', () => {
      const columns = Object.fromEntries(
        addressMeta.columns.map((c: any) => [c.propertyName, c.databaseName]),
      );

      expect(columns).toMatchObject({
        id: 'id',
        userId: 'user_id',
        title: 'title',
        street: 'street',
        city: 'city',
        state: 'state',
        postalCode: 'postal_code',
        latitude: 'latitude',
        longitude: 'longitude',
        isDefault: 'is_default',
        createdAt: 'created_at',
      });
    });

    it('defaults the isDefault flag to false', () => {
      const isDefault = addressMeta.columns.find(
        (c: any) => c.propertyName === 'isDefault',
      );

      expect(isDefault.default).toBe(false);
    });

    it('belongs to a user through the user_id foreign key', () => {
      const relation = addressMeta.relations.find(
        (r: any) => r.propertyName === 'user',
      );

      expect(relation).toBeDefined();
      expect(relation.relationType).toBe('many-to-one');
      expect(relation.inverseEntityMetadata.name).toBe('User');
      expect(relation.joinColumns[0].databaseName).toBe('user_id');
      expect(relation.onDelete).toBe('CASCADE');
    });
  });

  describe('Outbox', () => {
    it('maps to the outbox table with a generated uuid key', () => {
      const id = outboxMeta.columns.find((c: any) => c.propertyName === 'id');

      expect(outboxMeta.tableName).toBe('outbox');
      expect(id.isGenerated).toBe(true);
      expect(id.generationStrategy).toBe('uuid');
    });

    it('maps the event envelope columns', () => {
      const columns = Object.fromEntries(
        outboxMeta.columns.map((c: any) => [c.propertyName, c.databaseName]),
      );

      expect(columns).toMatchObject({
        id: 'id',
        aggregateType: 'aggregate_type',
        aggregateId: 'aggregate_id',
        eventType: 'event_type',
        payload: 'payload',
        processed: 'processed',
        createdAt: 'created_at',
      });
    });

    it('stores the payload as jsonb and defaults processed to false', () => {
      const payload = outboxMeta.columns.find(
        (c: any) => c.propertyName === 'payload',
      );
      const processed = outboxMeta.columns.find(
        (c: any) => c.propertyName === 'processed',
      );

      expect(payload.type).toBe('jsonb');
      expect(processed.default).toBe(false);
    });

    it('indexes the processing state together with the creation time', () => {
      const index = outboxMeta.indices.find(
        (i: any) =>
          i.columns.map((c: any) => c.propertyName).join(',') ===
          'processed,createdAt',
      );

      expect(index).toBeDefined();
      expect(index.name).toMatch(/^IDX_/);
      expect(index.isUnique).toBe(false);
      expect(index.columns.map((c: any) => c.propertyName)).toEqual([
        'processed',
        'createdAt',
      ]);
    });
  });

  describe('cross entity wiring', () => {
    it('resolves both sides of the user / address relation', () => {
      const userRelation = userMeta.relations.find(
        (r: any) => r.propertyName === 'addresses',
      );
      const addressRelation = addressMeta.relations.find(
        (r: any) => r.propertyName === 'user',
      );

      expect(userRelation.inverseEntityMetadata).toBe(addressMeta);
      expect(addressRelation.inverseEntityMetadata).toBe(userMeta);
      expect(userRelation.inverseRelation).toBe(addressRelation);
      expect(addressRelation.inverseRelation).toBe(userRelation);
    });
  });
});
