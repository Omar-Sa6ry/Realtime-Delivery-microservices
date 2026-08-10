export class ChangePasswordCommand {
  constructor(
    public readonly userId: string,
    public readonly passwordOld: string,
    public readonly passwordNew: string,
  ) {}
}
