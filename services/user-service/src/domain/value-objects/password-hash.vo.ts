export class PasswordHash {
  private readonly hash: string;

  constructor(hash: string) {
    if (!hash || hash.length < 10)
      throw new Error('Invalid password hash format');

    this.hash = hash;
  }

  public getValue(): string {
    return this.hash;
  }

  public equals(other: PasswordHash): boolean {
    return this.hash === other.getValue();
  }
}
