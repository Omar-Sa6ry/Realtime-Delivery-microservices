import { Injectable } from '@nestjs/common';
import { User } from '../../common/database/entities/user.entity';
import { Role } from '@delivery/common';
import { IdGenerator } from '@bts-soft/core';

@Injectable()
export class UserFactory {
  createUser(
    email: string,
    passwordHash: string,
    firstName: string,
    lastName: string,
    role: Role,
    phoneNumber?: string,
    imageUrl?: string,
  ): User {
    const user = new User();
    user.id = IdGenerator.generate('snowflake');
    user.email = email.toLowerCase().trim();
    user.passwordHash = passwordHash;
    user.firstName = firstName;
    user.lastName = lastName;
    user.phoneNumber = phoneNumber ? phoneNumber.trim() : '';
    user.role = role;
    user.isActive = true;
    user.imageUrl = imageUrl;
    user.addresses = [];
    return user;
  }
}
