export interface IPasswordHasher {
  hash(plainText: string): Promise<string>;
  compare(plainText: string, hash: string): Promise<boolean>;
}

export const IPASSWORD_HASHER = Symbol('IPasswordHasher');
