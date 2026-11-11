import { UserLookupService } from './user-lookup.service';
import { NotFoundException } from '@nestjs/common';
import { Role, USER_SERVICE_NAME } from '@delivery-micro/shard';

describe('UserLookupService', () => {
  let service: UserLookupService;
  let grpcClient: { getService: jest.Mock };
  let userGrpc: { getUser: jest.Mock };

  beforeEach(() => {
    userGrpc = { getUser: jest.fn() };
    grpcClient = {
      getService: jest.fn().mockReturnValue(userGrpc),
    };
    service = new UserLookupService(grpcClient as any);
    service.onModuleInit();
  });

  it('binds to the user service on module init', () => {
    expect(grpcClient.getService).toHaveBeenCalledWith(USER_SERVICE_NAME);
  });

  it('normalizes the gRPC response into an IUser', async () => {
    userGrpc.getUser.mockResolvedValue({
      id: 'u-1',
      email: 'a@b.com',
      role: 'driver',
      first_name: 'Ali',
      last_name: 'Hassan',
      is_active: false,
    });

    const user = await service.findById('u-1');

    expect(user).toMatchObject({
      id: 'u-1',
      email: 'a@b.com',
      role: Role.DRIVER,
      firstName: 'Ali',
      lastName: 'Hassan',
      isActive: false,
      isVerified: true,
    });
    expect(typeof user.createdAt).toBe('number');
  });

  it('maps the customer role to USER', async () => {
    userGrpc.getUser.mockResolvedValue({
      id: 'u-2',
      role: 'customer',
      email: 'c@d.com',
    });

    const user = await service.findById('u-2');

    expect(user.role).toBe(Role.USER);
  });

  it('defaults missing optional fields safely', async () => {
    userGrpc.getUser.mockResolvedValue({ id: 'u-3' });

    const user = await service.findById('u-3');

    expect(user).toMatchObject({
      id: 'u-3',
      email: '',
      firstName: '',
      lastName: '',
      isActive: true,
      isVerified: true,
      role: Role.USER,
    });
  });

  it('throws NotFoundException when the gRPC call rejects', async () => {
    userGrpc.getUser.mockRejectedValue(new Error('unavailable'));

    await expect(service.findById('u-4')).rejects.toThrow(NotFoundException);
    await expect(service.findById('u-4')).rejects.toThrow(
      'user.USER_NOT_FOUND',
    );
  });

  it('throws NotFoundException when the response has no id', async () => {
    userGrpc.getUser.mockResolvedValue({});

    await expect(service.findById('u-5')).rejects.toThrow(
      'user.USER_NOT_FOUND',
    );
  });

  it('throws NotFoundException when the response is null', async () => {
    userGrpc.getUser.mockResolvedValue(null);

    await expect(service.findById('u-6')).rejects.toThrow(
      'user.USER_NOT_FOUND',
    );
  });

  it('passes unknown roles through unchanged', async () => {
    userGrpc.getUser.mockResolvedValue({ id: 'u-7', role: 'admin' });

    const user = await service.findById('u-7');

    expect(user.role).toBe('admin');
  });
});
