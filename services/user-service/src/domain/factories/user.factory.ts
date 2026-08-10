import { Injectable } from '@nestjs/common';
import { UserAggregate } from '../aggregates/user.aggregate';
import { Email } from '../value-objects/email.vo';
import { PasswordHash } from '../value-objects/password-hash.vo';
import { PhoneNumber } from '../value-objects/phone-number.vo';
import { Role } from '@delivery/common';
import { IdGenerator } from '@bts-soft/core';

export interface CreateUserProps {
  email: string;
  passwordHash: string;
  firstName: string;
  lastName: string;
  phoneNumber?: string;
  role?: Role;
}

@Injectable()
export class UserFactory {
  public create(props: CreateUserProps): UserAggregate {
    return new UserAggregate({
      id: IdGenerator.generate('snowflake'),
      email: new Email(props.email),
      passwordHash: new PasswordHash(props.passwordHash),
      firstName: props.firstName,
      lastName: props.lastName,
      phoneNumber: props.phoneNumber ? new PhoneNumber(props.phoneNumber) : undefined,
      role: props.role || Role.USER,
      isActive: true,
      isVerified: false,
      createdAt: new Date(),
      updatedAt: new Date(),
    });
  }
}
