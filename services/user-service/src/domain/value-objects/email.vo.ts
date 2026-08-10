export class Email {
  private readonly value: string;

  constructor(email: string) {
    if (!email || !Email.isValid(email))
      throw new Error(`Invalid email address: ${email}`);

    this.value = email.toLowerCase().trim();
  }

  public getValue(): string {
    return this.value;
  }

  public equals(other: Email): boolean {
    return this.value === other.getValue();
  }

  public static isValid(email: string): boolean {
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return emailRegex.test(email);
  }

  public toString(): string {
    return this.value;
  }
}
