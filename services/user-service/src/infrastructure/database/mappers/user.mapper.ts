import { UserOrmEntity } from '../entities/user.orm-entity';
import { AddressOrmEntity } from '../entities/address.orm-entity';
import { UserAggregate } from '../../../domain/aggregates/user.aggregate';
import { AddressEntity } from '../../../domain/entities/address.entity';
import { Email } from '../../../domain/value-objects/email.vo';
import { PasswordHash } from '../../../domain/value-objects/password-hash.vo';
import { PhoneNumber } from '../../../domain/value-objects/phone-number.vo';

export class UserMapper {
  public static toDomain(orm: UserOrmEntity): UserAggregate {
    const addresses = (orm.addresses || []).map(
      (addr) =>
        new AddressEntity({
          id: addr.id,
          userId: addr.userId,
          title: addr.title,
          street: addr.street,
          city: addr.city,
          state: addr.state,
          postalCode: addr.postalCode,
          latitude: addr.latitude,
          longitude: addr.longitude,
          isDefault: addr.isDefault,
          createdAt: addr.createdAt,
        }),
    );

    return new UserAggregate({
      id: orm.id,
      email: new Email(orm.email),
      passwordHash: new PasswordHash(orm.passwordHash),
      firstName: orm.firstName,
      lastName: orm.lastName,
      phoneNumber: orm.phoneNumber ? new PhoneNumber(orm.phoneNumber) : undefined,
      role: orm.role,
      isActive: orm.isActive,
      isVerified: orm.isVerified,
      resetToken: orm.resetToken,
      resetTokenExpiry: orm.resetTokenExpiry,
      addresses,
      createdAt: orm.createdAt,
      updatedAt: orm.updatedAt,
    });
  }

  public static toOrm(domain: UserAggregate): UserOrmEntity {
    const orm = new UserOrmEntity();
    orm.id = domain.getId();
    orm.email = domain.getEmail().getValue();
    orm.passwordHash = domain.getPasswordHash().getValue();
    orm.firstName = domain.getFirstName();
    orm.lastName = domain.getLastName();
    orm.phoneNumber = domain.getPhoneNumber()?.getValue() || undefined;
    orm.role = domain.getRole();
    orm.isActive = domain.getIsActive();
    orm.isVerified = domain.getIsVerified();
    orm.resetToken = domain.getResetToken();
    orm.resetTokenExpiry = domain.getResetTokenExpiry();
    orm.createdAt = domain.getCreatedAt();
    orm.updatedAt = domain.getUpdatedAt();

    orm.addresses = domain.getAddresses().map((addr) => {
      const addrOrm = new AddressOrmEntity();
      addrOrm.id = addr.getId();
      addrOrm.userId = domain.getId();
      addrOrm.title = addr.getTitle();
      addrOrm.street = addr.getStreet();
      addrOrm.city = addr.getCity();
      addrOrm.state = addr.getState();
      addrOrm.postalCode = addr.getPostalCode();
      addrOrm.latitude = addr.getLatitude();
      addrOrm.longitude = addr.getLongitude();
      addrOrm.isDefault = addr.getIsDefault();
      addrOrm.createdAt = addr.getCreatedAt();
      return addrOrm;
    });

    return orm;
  }
}
