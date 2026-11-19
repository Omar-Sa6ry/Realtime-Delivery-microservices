import { plainToInstance } from 'class-transformer';
import { validate, ValidationError } from 'class-validator';
import {
  AddAddressInput,
  AddressListResponse,
  AddressResponse,
  AddressType,
  ChangePasswordInput,
  PaginatedUsers,
  PaginatedUsersResponse,
  UpdateProfileInput,
  UserResponse,
  UserType,
} from './user.types';

const validAddress = {
  title: 'Home',
  street: '1 Main Street',
  city: 'Cairo',
  state: 'Cairo',
  postalCode: '11511',
  latitude: 30.0444,
  longitude: 31.2357,
  isDefault: true,
};

/** Validate a plain payload against a DTO class and return the raw errors. */
async function validatePlain<T extends object>(
  type: new () => T,
  plain: Record<string, unknown>,
): Promise<ValidationError[]> {
  return validate(plainToInstance(type, plain), {
    whitelist: false,
    forbidUnknownValues: false,
  });
}

/** One entry per failing property, e.g. `street must be between ...`. */
function problems(errors: ValidationError[]): string[] {
  return errors.map(
    (e) => `${e.property}: ${Object.values(e.constraints ?? {}).join(' | ')}`,
  );
}

describe('user DTOs', () => {
  describe('ChangePasswordInput', () => {
    it('accepts a well formed old/new password pair', async () => {
      const instance = plainToInstance(ChangePasswordInput, {
        passwordOld: 'OldPass123',
        passwordNew: 'NewPass456',
      });

      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('requires both passwords', async () => {
      const errors = await validatePlain(ChangePasswordInput, {});

      expect(errors.map((e) => e.property).sort()).toEqual([
        'passwordNew',
        'passwordOld',
      ]);
    });

    it('rejects a new password below the eight character minimum', async () => {
      const errors = await validatePlain(ChangePasswordInput, {
        passwordOld: 'OldPass123',
        passwordNew: 'Ab1',
      });

      expect(problems(errors).join(' ')).toContain(
        'between 8 and 30 characters',
      );
    });

    it('rejects a new password without upper, lower and digit characters', async () => {
      const errors = await validatePlain(ChangePasswordInput, {
        passwordOld: 'OldPass123',
        passwordNew: 'alllowercase',
      });

      expect(problems(errors).join(' ')).toContain(
        'uppercase, lowercase and number',
      );
    });

    it('rejects a new password longer than thirty characters', async () => {
      const errors = await validatePlain(ChangePasswordInput, {
        passwordOld: 'OldPass123',
        passwordNew: 'Aa1'.repeat(11),
      });

      expect(problems(errors).join(' ')).toContain(
        'between 8 and 30 characters',
      );
    });
  });

  describe('UpdateProfileInput', () => {
    it('accepts a partial profile update', async () => {
      const instance = plainToInstance(UpdateProfileInput, {
        firstName: 'Jane',
        lastName: 'Smith',
      });

      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('accepts an empty update (every field is optional)', async () => {
      await expect(
        validate(plainToInstance(UpdateProfileInput, {})),
      ).resolves.toHaveLength(0);
    });

    it('lower-cases the name fields through the shared text transform', () => {
      const instance = plainToInstance(UpdateProfileInput, {
        firstName: 'JANE',
        lastName: 'Smith',
      });

      expect(instance.firstName).toBe('jane');
      expect(instance.lastName).toBe('smith');
    });

    it('rejects a first name shorter than two characters', async () => {
      const errors = await validatePlain(UpdateProfileInput, {
        firstName: 'J',
      });

      expect(problems(errors).join(' ')).toContain(
        'between 2 and 100 characters',
      );
    });

    it('rejects a first name with unsupported symbols', async () => {
      const errors = await validatePlain(UpdateProfileInput, {
        firstName: 'Jane!',
      });

      expect(problems(errors).join(' ')).toContain(
        'only letters, numbers, spaces',
      );
    });

    it('rejects a non string avatar url', async () => {
      const errors = await validatePlain(UpdateProfileInput, { imageUrl: 42 });

      expect(problems(errors).join(' ')).toContain('must be a string');
    });

    it('rejects a non string avatar media id', async () => {
      const errors = await validatePlain(UpdateProfileInput, {
        avatarMediaId: { id: 1 },
      });

      expect(problems(errors).join(' ')).toContain('must be a string');
    });
  });

  describe('AddAddressInput', () => {
    it('accepts a complete address', async () => {
      const instance = plainToInstance(AddAddressInput, validAddress);

      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('accepts an address without the optional fields', async () => {
      const instance = plainToInstance(AddAddressInput, {
        title: 'Office',
        street: '2 Nile Street',
        city: 'Giza',
      });

      await expect(validate(instance)).resolves.toHaveLength(0);
      expect(instance.isDefault).toBeUndefined();
    });

    it('requires the title, street and city', async () => {
      const errors = await validatePlain(AddAddressInput, {});

      expect(errors.map((e) => e.property).sort()).toEqual([
        'city',
        'street',
        'title',
      ]);
    });

    it('rejects a title shorter than two characters', async () => {
      const errors = await validatePlain(AddAddressInput, {
        ...validAddress,
        title: 'H',
      });

      expect(problems(errors).join(' ')).toContain(
        'between 2 and 100 characters',
      );
    });

    it('rejects a street longer than two hundred characters', async () => {
      const errors = await validatePlain(AddAddressInput, {
        ...validAddress,
        street: 'x'.repeat(201),
      });

      expect(problems(errors).join(' ')).toContain(
        'between 2 and 200 characters',
      );
    });

    it('rejects a non numeric latitude', async () => {
      const errors = await validatePlain(AddAddressInput, {
        ...validAddress,
        latitude: 'north',
      });

      expect(problems(errors).join(' ')).toContain('must be a number');
    });

    it('rejects a non numeric longitude', async () => {
      const errors = await validatePlain(AddAddressInput, {
        ...validAddress,
        longitude: 'east',
      });

      expect(problems(errors).join(' ')).toContain('must be a number');
    });

    it('rejects a non boolean isDefault flag', async () => {
      const errors = await validatePlain(AddAddressInput, {
        ...validAddress,
        isDefault: 'yes',
      });

      expect(problems(errors).join(' ')).toContain('must be a boolean');
    });

    it('rejects a postal code longer than twenty characters', async () => {
      const errors = await validatePlain(AddAddressInput, {
        ...validAddress,
        postalCode: '1'.repeat(21),
      });

      expect(problems(errors).join(' ')).toContain(
        'between 2 and 20 characters',
      );
    });
  });

  describe('response envelopes', () => {
    it('exposes the shared success fields on every user response', () => {
      const response = new UserResponse();

      expect(response).toHaveProperty('success', true);
      expect(response).toHaveProperty('statusCode', 200);
      expect(response).toHaveProperty(
        'message',
        'Operation executed successfully',
      );
      expect(typeof response.timeStamp).toBe('string');
    });

    it('declares the data envelope field on the user responses', () => {
      // `data` is contributed by the shared `GeneralResponse` base class: its
      // property decorators only register GraphQL metadata, so the field is
      // observable through the emitted `design:type` annotation instead of as
      // an own property of a freshly constructed instance.
      expect(
        Reflect.getMetadata('design:type', UserResponse.prototype, 'data'),
      ).toBeDefined();

      const response = new UserResponse() as any;
      response.data = new UserType();

      expect(response.data).toBeInstanceOf(UserType);
    });

    it('exposes the shared success fields on address responses', () => {
      expect(new AddressResponse()).toHaveProperty('success', true);
      expect(new AddressListResponse()).toHaveProperty('statusCode', 200);
      expect(new PaginatedUsersResponse()).toHaveProperty(
        'message',
        'Operation executed successfully',
      );
    });

    it('declares the data envelope field on the address responses', () => {
      expect(
        Reflect.getMetadata('design:type', AddressResponse.prototype, 'data'),
      ).toBeDefined();
      // `AddressListResponse` re-declares `data` as a list of addresses.
      expect(
        Reflect.getMetadata(
          'design:type',
          AddressListResponse.prototype,
          'data',
        ),
      ).toBe(Array);

      const list = new AddressListResponse();
      list.data = [new AddressType()];

      expect(list.data).toHaveLength(1);
    });

    it('builds paginated user collections', () => {
      const page = new PaginatedUsers();

      page.items = [];
      page.total = 0;

      expect(page.items).toEqual([]);
      expect(page.total).toBe(0);
    });

    it('builds entity objects with the federation key fields', () => {
      const user = new UserType();
      user.id = 'user-1';
      user.email = 'john@example.com';
      user.addresses = [];

      const address = new AddressType();
      address.id = 'addr-1';
      address.userId = 'user-1';
      address.isDefault = false;

      expect(user.id).toBe('user-1');
      expect(address.userId).toBe('user-1');
    });
  });
});
