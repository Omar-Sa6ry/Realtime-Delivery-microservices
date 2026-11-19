import { Test } from '@nestjs/testing';
import {
  GraphQLSchemaBuilderModule,
  GraphQLSchemaFactory,
} from '@nestjs/graphql';
import { printSchema } from 'graphql';
import { AppResolver } from './app.resolver';
import { UserResolver } from './modules/user/user.resolver';
import { AuthResolver } from './modules/auth/auth.resolver';
import { PaginatedUsersResponse } from './modules/user/dto/user.types';

/**
 * Building the executable schema is the only code path that resolves the
 * lazy `() => SomeType` thunks handed to `@Query`, `@Mutation` and `@Field`,
 * so this suite exercises every DTO and resolver type reference in one go and
 * asserts the published contract of the service.
 */
describe('GraphQL schema', () => {
  let sdl: string;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      imports: [GraphQLSchemaBuilderModule],
    }).compile();

    const factory = moduleRef.get(GraphQLSchemaFactory);
    const schema = await factory.create(
      [AppResolver, UserResolver, AuthResolver],
      [],
      {
        // `PaginatedUsersResponse` is not reachable from any resolver yet, so
        // it has to be registered as an orphaned type to stay in the schema.
        orphanedTypes: [PaginatedUsersResponse],
      },
    );

    sdl = printSchema(schema);
    await moduleRef.close();
  });

  it('publishes the query and mutation roots', () => {
    expect(sdl).toContain('type Query');
    expect(sdl).toContain('type Mutation');
  });

  it('exposes the service health probe', () => {
    expect(sdl).toContain('pingForUser: BooleanResponse!');
  });

  it('exposes the profile queries', () => {
    expect(sdl).toContain('myProfile: UserResponse!');
    expect(sdl).toContain('myAddresses: AddressListResponse!');
    expect(sdl).toContain('user(id: String!): UserResponse!');
  });

  it('exposes the profile mutations', () => {
    expect(sdl).toContain(
      'changePassword(input: ChangePasswordInput!): BooleanResponse!',
    );
    expect(sdl).toContain(
      'updateProfile(input: UpdateProfileInput!): UserResponse!',
    );
    expect(sdl).toContain(
      'addAddress(input: AddAddressInput!): AddressResponse!',
    );
    expect(sdl).toContain(
      'deleteAddress(addressId: String!): BooleanResponse!',
    );
    expect(sdl).toContain(
      'setDefaultAddress(addressId: String!): BooleanResponse!',
    );
  });

  it('exposes the administration mutations behind permissions', () => {
    expect(sdl).toContain('promoteUserToAdmin(id: String!): BooleanResponse!');
    expect(sdl).toContain(
      'toggleUserActive(id: String!, isActive: Boolean!): BooleanResponse!',
    );
    expect(sdl).toContain('deleteUser(id: String!): BooleanResponse!');
  });

  it('exposes the authentication mutations', () => {
    expect(sdl).toContain('register(input: RegisterInput!): AuthResponse!');
    expect(sdl).toContain('login(input: LoginInput!): AuthResponse!');
    expect(sdl).toContain(
      'forgetPassword(input: ForgetPasswordInput!): BooleanResponse!',
    );
    expect(sdl).toContain(
      'resetPassword(input: ResetPasswordInput!): BooleanResponse!',
    );
    expect(sdl).toContain(
      'refreshToken(input: RefreshTokenInput!): AuthResponse!',
    );
    expect(sdl).toContain('logout: BooleanResponse!');
  });

  it('declares the federated User entity with its key field', () => {
    // The `@key` directive itself is contributed by the Apollo federation
    // driver at runtime; the schema factory only proves the entity shape.
    expect(sdl).toContain('type User {');
    expect(sdl).toContain('id: ID!');
    expect(sdl).toContain('addresses: [AddressType!]!');
  });

  it('declares the shared response envelopes', () => {
    expect(sdl).toContain('type UserResponse');
    expect(sdl).toContain('type AddressResponse');
    expect(sdl).toContain('type AddressListResponse');
    expect(sdl).toContain('type PaginatedUsersResponse');
    expect(sdl).toContain('type AuthResponse');
    expect(sdl).toContain('type BooleanResponse');
  });

  it('declares the address input and entity types', () => {
    expect(sdl).toContain('input AddAddressInput');
    expect(sdl).toContain('type AddressType');
    expect(sdl).toContain('input ChangePasswordInput');
    expect(sdl).toContain('input UpdateProfileInput');
    expect(sdl).toContain('input RegisterInput');
    expect(sdl).toContain('input LoginInput');
    expect(sdl).toContain('input ForgetPasswordInput');
    expect(sdl).toContain('input ResetPasswordInput');
    expect(sdl).toContain('input RefreshTokenInput');
  });
});
