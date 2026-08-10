import {
  Entity,
  PrimaryColumn,
  Column,
  CreateDateColumn,
  UpdateDateColumn,
  OneToMany,
} from 'typeorm';
import { AddressOrmEntity } from './address.orm-entity';
import { Role } from '@delivery/common';

@Entity('users')
export class UserOrmEntity {
  @PrimaryColumn({ type: 'varchar', length: 36 })
  id: string;

  @Column({ unique: true })
  email: string;

  @Column({ name: 'password_hash' })
  passwordHash: string;

  @Column({ name: 'first_name' })
  firstName: string;

  @Column({ name: 'last_name' })
  lastName: string;

  @Column({ name: 'phone_number', nullable: true })
  phoneNumber?: string;

  @Column({ type: 'varchar', default: Role.USER })
  role: Role;

  @Column({ name: 'is_active', default: true })
  isActive: boolean;

  @Column({ name: 'is_verified', default: false })
  isVerified: boolean;

  @Column({ name: 'reset_token', nullable: true })
  resetToken?: string;

  @Column({ name: 'reset_token_expiry', type: 'timestamptz', nullable: true })
  resetTokenExpiry?: Date;

  @OneToMany(() => AddressOrmEntity, (address) => address.user, {
    cascade: true,
    eager: true,
  })
  addresses: AddressOrmEntity[];

  @CreateDateColumn({ name: 'created_at' })
  createdAt: Date;

  @UpdateDateColumn({ name: 'updated_at' })
  updatedAt: Date;
}
