import { Email } from '../value-objects/email.vo';
import { PasswordHash } from '../value-objects/password-hash.vo';
import { PhoneNumber } from '../value-objects/phone-number.vo';
import { AddressEntity } from '../entities/address.entity';
import { Role } from '@delivery/common';
import { IdGenerator } from '@bts-soft/core';

export interface UserAggregateProps {
  id?: string;
  email: Email;
  passwordHash: PasswordHash;
  firstName: string;
  lastName: string;
  phoneNumber?: PhoneNumber;
  role?: Role;
  isActive?: boolean;
  isVerified?: boolean;
  resetToken?: string;
  resetTokenExpiry?: Date;
  addresses?: AddressEntity[];
  createdAt?: Date;
  updatedAt?: Date;
}

export class UserAggregate {
  private readonly id: string;
  private email: Email;
  private passwordHash: PasswordHash;
  private firstName: string;
  private lastName: string;
  private phoneNumber?: PhoneNumber;
  private role: Role;
  private isActive: boolean;
  private isVerified: boolean;
  private resetToken?: string;
  private resetTokenExpiry?: Date;
  private addresses: AddressEntity[];
  private readonly createdAt: Date;
  private updatedAt: Date;

  private domainEvents: any[] = [];

  constructor(props: UserAggregateProps) {
    this.id = props.id || IdGenerator.generate('snowflake');
    this.email = props.email;
    this.passwordHash = props.passwordHash;
    this.firstName = props.firstName;
    this.lastName = props.lastName;
    this.phoneNumber = props.phoneNumber;
    this.role = props.role || Role.USER;
    this.isActive = props.isActive ?? true;
    this.isVerified = props.isVerified ?? false;
    this.resetToken = props.resetToken;
    this.resetTokenExpiry = props.resetTokenExpiry;
    this.addresses = props.addresses || [];
    this.createdAt = props.createdAt || new Date();
    this.updatedAt = props.updatedAt || new Date();
  }

  // Getters
  public getId(): string { return this.id; }
  public getEmail(): Email { return this.email; }
  public getPasswordHash(): PasswordHash { return this.passwordHash; }
  public getFirstName(): string { return this.firstName; }
  public getLastName(): string { return this.lastName; }
  public getFullName(): string { return `${this.firstName} ${this.lastName}`; }
  public getPhoneNumber(): PhoneNumber | undefined { return this.phoneNumber; }
  public getRole(): Role { return this.role; }
  public getIsActive(): boolean { return this.isActive; }
  public getIsVerified(): boolean { return this.isVerified; }
  public getResetToken(): string | undefined { return this.resetToken; }
  public getResetTokenExpiry(): Date | undefined { return this.resetTokenExpiry; }
  public getAddresses(): AddressEntity[] { return [...this.addresses]; }
  public getCreatedAt(): Date { return this.createdAt; }
  public getUpdatedAt(): Date { return this.updatedAt; }

  // Domain Business Methods
  public updateProfile(firstName?: string, lastName?: string, phoneNumber?: PhoneNumber) {
    if (firstName) this.firstName = firstName;
    if (lastName) this.lastName = lastName;
    if (phoneNumber !== undefined) this.phoneNumber = phoneNumber;
    this.updatedAt = new Date();
  }

  public changePassword(newPasswordHash: PasswordHash) {
    this.passwordHash = newPasswordHash;
    this.resetToken = undefined;
    this.resetTokenExpiry = undefined;
    this.updatedAt = new Date();
  }

  public generateResetToken(token: string, expiryDurationMinutes: number = 15) {
    this.resetToken = token;
    this.resetTokenExpiry = new Date(Date.now() + expiryDurationMinutes * 60 * 1000);
    this.updatedAt = new Date();
  }

  public clearResetToken() {
    this.resetToken = undefined;
    this.resetTokenExpiry = undefined;
    this.updatedAt = new Date();
  }

  public isResetTokenValid(token: string): boolean {
    if (!this.resetToken || this.resetToken !== token) return false;
    if (!this.resetTokenExpiry || this.resetTokenExpiry.getTime() < Date.now()) return false;
    return true;
  }

  public changeRole(newRole: Role) {
    this.role = newRole;
    this.updatedAt = new Date();
  }

  public addAddress(address: AddressEntity) {
    if (address.getIsDefault()) {
      this.addresses.forEach(a => a.setDefault(false));
    }
    this.addresses.push(address);
    this.updatedAt = new Date();
  }

  public getDomainEvents(): any[] {
    return this.domainEvents;
  }

  public clearDomainEvents() {
    this.domainEvents = [];
  }

  public addDomainEvent(event: any) {
    this.domainEvents.push(event);
  }
}
