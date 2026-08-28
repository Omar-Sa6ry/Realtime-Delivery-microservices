import { Column } from 'typeorm';

export class Address {
  @Column({ type: 'varchar', length: 160 })
  line1!: string;

  @Column({ type: 'varchar', length: 160, nullable: true })
  line2!: string | null;

  @Column({ type: 'varchar', length: 80 })
  city!: string;

  @Column({ type: 'varchar', length: 80, nullable: true })
  state!: string | null;

  @Column({ type: 'varchar', length: 20, nullable: true })
  postalCode!: string | null;

  @Column({ type: 'varchar', length: 2, default: 'US' })
  countryCode!: string;

  @Column({ type: 'decimal', precision: 10, scale: 7, nullable: true })
  latitude!: number | null;

  @Column({ type: 'decimal', precision: 10, scale: 7, nullable: true })
  longitude!: number | null;
}
