import { plainToInstance } from 'class-transformer';
import { validate, ValidationError } from 'class-validator';
import {
  AuthPayloadType,
  AuthResponse,
  ForgetPasswordInput,
  LoginInput,
  RefreshTokenInput,
  RegisterInput,
  ResetPasswordInput,
} from './auth.types';

const validRegister = {
  email: 'john@example.com',
  password: 'Passw0rd1',
  firstName: 'John',
  lastName: 'Doe',
  phoneNumber: '01000000000',
};

async function validatePlain<T extends object>(
  type: new () => T,
  plain: Record<string, unknown>,
): Promise<ValidationError[]> {
  return validate(plainToInstance(type, plain), {
    whitelist: false,
    forbidUnknownValues: false,
  });
}

function problems(errors: ValidationError[]): string[] {
  return errors.map(
    (e) => `${e.property}: ${Object.values(e.constraints ?? {}).join(' | ')}`,
  );
}

describe('auth DTOs', () => {
  describe('RegisterInput', () => {
    it('accepts a complete registration payload', async () => {
      const instance = plainToInstance(RegisterInput, validRegister);

      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('accepts a registration without the optional image url', async () => {
      const withoutImage: Record<string, unknown> = { ...validRegister };
      delete withoutImage.imageUrl;
      const instance = plainToInstance(RegisterInput, withoutImage);

      expect(withoutImage.imageUrl).toBeUndefined();
      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('lower-cases the e-mail address', () => {
      const instance = plainToInstance(RegisterInput, {
        ...validRegister,
        email: 'JOHN@Example.COM',
      });

      expect(instance.email).toBe('john@example.com');
    });

    it('title-cases the name fields', () => {
      const instance = plainToInstance(RegisterInput, {
        ...validRegister,
        firstName: 'jOHN',
        lastName: 'dOE',
      });

      expect(instance.firstName).toBe('John');
      expect(instance.lastName).toBe('Doe');
    });

    it('strips formatting characters from the phone number', () => {
      const instance = plainToInstance(RegisterInput, {
        ...validRegister,
        phoneNumber: '010 0000 0000',
      });

      expect(instance.phoneNumber).toBe('01000000000');
    });

    it('keeps a leading plus sign while stripping everything else', () => {
      const instance = plainToInstance(RegisterInput, {
        ...validRegister,
        phoneNumber: '+20-100-000-0000',
      });

      expect(instance.phoneNumber).toBe('+201000000000');
    });

    it('rejects a malformed e-mail address', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        email: 'not-an-email',
      });

      expect(problems(errors).join(' ')).toContain(
        'Must be a valid email address',
      );
    });

    it('requires the e-mail address', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        email: undefined,
      });

      expect(errors.map((e) => e.property)).toContain('email');
    });

    it('rejects a password that is too short', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        password: 'Ab1',
      });

      expect(problems(errors).join(' ')).toContain(
        'between 8 and 30 characters',
      );
    });

    it('rejects a password without upper, lower and digit characters', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        password: 'abcdefghij',
      });

      expect(problems(errors).join(' ')).toContain(
        'uppercase, lowercase and number',
      );
    });

    it('rejects a first name containing digits', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        firstName: 'J0hn',
      });

      expect(problems(errors).join(' ')).toContain('only letters and spaces');
    });

    it('rejects a first name shorter than two characters', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        firstName: 'J',
      });

      expect(problems(errors).join(' ')).toContain(
        'between 2 and 100 characters',
      );
    });

    it('rejects a phone number that is not a valid Egyptian number', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        phoneNumber: '12345',
      });

      expect(problems(errors).join(' ')).toContain('must be a valid EG number');
    });

    it('requires the phone number', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        phoneNumber: undefined,
      });

      expect(errors.map((e) => e.property)).toContain('phoneNumber');
    });

    it('rejects a non string image url', async () => {
      const errors = await validatePlain(RegisterInput, {
        ...validRegister,
        imageUrl: 123,
      });

      expect(problems(errors).join(' ')).toContain('must be a string');
    });
  });

  describe('LoginInput', () => {
    it('accepts a valid e-mail and password pair', async () => {
      const instance = plainToInstance(LoginInput, {
        email: 'john@example.com',
        password: 'Passw0rd1',
      });

      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('rejects a malformed e-mail address', async () => {
      const errors = await validatePlain(LoginInput, {
        email: 'john@',
        password: 'Passw0rd1',
      });

      expect(problems(errors).join(' ')).toContain(
        'Must be a valid email address',
      );
    });

    it('requires the password', async () => {
      const errors = await validatePlain(LoginInput, {
        email: 'john@example.com',
      });

      expect(errors.map((e) => e.property)).toContain('password');
    });

    it('lower-cases the e-mail address', () => {
      const instance = plainToInstance(LoginInput, {
        email: 'JOHN@EXAMPLE.COM',
        password: 'Passw0rd1',
      });

      expect(instance.email).toBe('john@example.com');
    });
  });

  describe('ForgetPasswordInput', () => {
    it('accepts a valid e-mail address', async () => {
      const instance = plainToInstance(ForgetPasswordInput, {
        email: 'john@example.com',
      });

      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('rejects a malformed e-mail address', async () => {
      const errors = await validatePlain(ForgetPasswordInput, {
        email: 'nope',
      });

      expect(problems(errors).join(' ')).toContain(
        'Must be a valid email address',
      );
    });
  });

  describe('ResetPasswordInput', () => {
    it('accepts a six digit token and a compliant new password', async () => {
      const instance = plainToInstance(ResetPasswordInput, {
        token: '123456',
        passwordNew: 'BrandNew1',
      });

      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('requires the token', async () => {
      const errors = await validatePlain(ResetPasswordInput, {
        passwordNew: 'BrandNew1',
      });

      expect(errors.map((e) => e.property)).toContain('token');
    });

    it('rejects an empty token', async () => {
      const errors = await validatePlain(ResetPasswordInput, {
        token: '',
        passwordNew: 'BrandNew1',
      });

      expect(problems(errors).join(' ')).toContain(
        'between 1 and 100 characters',
      );
    });

    it('rejects a new password that is too short', async () => {
      const errors = await validatePlain(ResetPasswordInput, {
        token: '123456',
        passwordNew: 'Ab1',
      });

      expect(problems(errors).join(' ')).toContain(
        'between 8 and 30 characters',
      );
    });

    it('lower-cases nothing on the token (the code is numeric)', () => {
      const instance = plainToInstance(ResetPasswordInput, {
        token: '123456',
        passwordNew: 'BrandNew1',
      });

      expect(instance.token).toBe('123456');
    });
  });

  describe('RefreshTokenInput', () => {
    it('accepts a long opaque refresh token', async () => {
      const instance = plainToInstance(RefreshTokenInput, {
        refreshToken: 'x'.repeat(40),
      });

      await expect(validate(instance)).resolves.toHaveLength(0);
    });

    it('accepts a base64url JWT (no character normalisation happens)', () => {
      const raw = 'a-b_c.d-e_f';
      const instance = plainToInstance(RefreshTokenInput, {
        refreshToken: raw,
      });

      expect(instance.refreshToken).toBe(raw);
    });

    it('rejects a token shorter than ten characters', async () => {
      const errors = await validatePlain(RefreshTokenInput, {
        refreshToken: 'short',
      });

      expect(problems(errors).join(' ')).toContain(
        'must be longer than or equal to 10 characters',
      );
    });

    it('rejects a non string token', async () => {
      const errors = await validatePlain(RefreshTokenInput, {
        refreshToken: 12345678901,
      });

      expect(problems(errors).join(' ')).toContain('must be a string');
    });

    it('requires the token', async () => {
      const errors = await validatePlain(RefreshTokenInput, {});

      expect(errors.map((e) => e.property)).toContain('refreshToken');
    });
  });

  describe('AuthPayloadType / AuthResponse', () => {
    it('carries the user and the token pair', () => {
      const payload = new AuthPayloadType();

      payload.accessToken = 'access';
      payload.refreshToken = 'refresh';
      payload.user = { id: 'user-1' } as AuthPayloadType['user'];

      expect(payload.accessToken).toBe('access');
      expect(payload.refreshToken).toBe('refresh');
      expect(payload.user.id).toBe('user-1');
    });

    it('exposes the shared success envelope on the auth response', () => {
      const response = new AuthResponse();

      expect(response).toHaveProperty('success', true);
      expect(response).toHaveProperty('statusCode', 200);
      expect(response).toHaveProperty(
        'message',
        'Operation executed successfully',
      );
      expect(typeof response.timeStamp).toBe('string');
    });

    it('declares the data envelope field on the auth response', () => {
      // `data` comes from the shared `GeneralResponse` base class; its
      // property decorators only register GraphQL metadata, so the field is
      // observable through the emitted `design:type` annotation.
      expect(
        Reflect.getMetadata('design:type', AuthResponse.prototype, 'data'),
      ).toBeDefined();

      const response = new AuthResponse() as any;
      response.data = { accessToken: 'access' };

      expect(response.data.accessToken).toBe('access');
    });
  });
});
