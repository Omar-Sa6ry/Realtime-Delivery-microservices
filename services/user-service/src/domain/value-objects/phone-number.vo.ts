export class PhoneNumber {
  private readonly value: string;

  constructor(phoneNumber?: string) {
    if (phoneNumber && !PhoneNumber.isValid(phoneNumber)) {
      throw new Error(`Invalid phone number: ${phoneNumber}`);
    }
    this.value = phoneNumber ? phoneNumber.trim() : "";
  }

  public getValue(): string {
    return this.value;
  }

  public static isValid(phone: string): boolean {
    const phoneRegex = /^\+?[1-9]\d{1,14}$/;
    return phoneRegex.test(phone.replace(/[\s-]/g, ""));
  }
}
