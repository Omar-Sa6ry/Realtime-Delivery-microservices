import { IdGenerator } from '@bts-soft/core';

export interface AddressProps {
  id?: string;
  userId: string;
  title: string;
  street: string;
  city: string;
  state?: string;
  postalCode?: string;
  latitude?: number;
  longitude?: number;
  isDefault?: boolean;
  createdAt?: Date;
}

export class AddressEntity {
  private readonly id: string;
  private readonly userId: string;
  private title: string;
  private street: string;
  private city: string;
  private state?: string;
  private postalCode?: string;
  private latitude?: number;
  private longitude?: number;
  private isDefault: boolean;
  private readonly createdAt: Date;

  constructor(props: AddressProps) {
    this.id = props.id || IdGenerator.generate('snowflake');
    this.userId = props.userId;
    this.title = props.title;
    this.street = props.street;
    this.city = props.city;
    this.state = props.state;
    this.postalCode = props.postalCode;
    this.latitude = props.latitude;
    this.longitude = props.longitude;
    this.isDefault = props.isDefault ?? false;
    this.createdAt = props.createdAt || new Date();
  }

  public getId(): string { return this.id; }
  public getUserId(): string { return this.userId; }
  public getTitle(): string { return this.title; }
  public getStreet(): string { return this.street; }
  public getCity(): string { return this.city; }
  public getState(): string | undefined { return this.state; }
  public getPostalCode(): string | undefined { return this.postalCode; }
  public getLatitude(): number | undefined { return this.latitude; }
  public getLongitude(): number | undefined { return this.longitude; }
  public getIsDefault(): boolean { return this.isDefault; }
  public getCreatedAt(): Date { return this.createdAt; }

  public update(props: Partial<Omit<AddressProps, 'id' | 'userId' | 'createdAt'>>) {
    if (props.title !== undefined) this.title = props.title;
    if (props.street !== undefined) this.street = props.street;
    if (props.city !== undefined) this.city = props.city;
    if (props.state !== undefined) this.state = props.state;
    if (props.postalCode !== undefined) this.postalCode = props.postalCode;
    if (props.latitude !== undefined) this.latitude = props.latitude;
    if (props.longitude !== undefined) this.longitude = props.longitude;
    if (props.isDefault !== undefined) this.isDefault = props.isDefault;
  }

  public setDefault(isDefault: boolean) {
    this.isDefault = isDefault;
  }
}
