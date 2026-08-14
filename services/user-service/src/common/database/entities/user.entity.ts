import {
  Entity,
  PrimaryColumn,
  Column,
  CreateDateColumn,
  UpdateDateColumn,
  OneToMany,
} from 'typeorm';
import { Address } from './address.entity';
import { Role } from '@delivery/common';

@Entity('users')
export class User {
  @PrimaryColumn({ type: 'varchar', length: 36 })
  id: string;

  @Column({ unique: true, length: 255 })
  email: string;

  @Column({ name: 'password_hash', length: 255 })
  passwordHash: string;

  @Column({ name: 'first_name', length: 100 })
  firstName: string;

  @Column({ name: 'last_name', length: 100 })
  lastName: string;

  @Column({ name: 'phone_number', unique: true, length: 20 })
  phoneNumber: string;

  @Column({ type: 'varchar', default: Role.USER })
  role: Role;

  @Column({ name: 'is_active', default: true })
  isActive: boolean;

  @Column({ name: 'reset_token', nullable: true })
  resetToken?: string;

  @Column({ name: 'reset_token_expiry', type: 'timestamptz', nullable: true })
  resetTokenExpiry?: Date;

  @Column({
    name: 'image_url',
    type: 'varchar',
    length: 1024,
    nullable: true,
    default: null,
  })
  imageUrl?: string;

  @OneToMany(() => Address, (address) => address.user, {
    cascade: true,
    eager: true,
  })
  addresses: Address[];

  @CreateDateColumn({ name: 'created_at' })
  createdAt: Date;

  @UpdateDateColumn({ name: 'updated_at' })
  updatedAt: Date;
}
