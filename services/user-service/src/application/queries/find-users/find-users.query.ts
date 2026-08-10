export class FindUsersQuery {
  constructor(
    public readonly page: number,
    public readonly limit: number,
  ) {}
}
