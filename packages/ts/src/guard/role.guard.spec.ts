import { UnauthorizedException } from '@nestjs/common';
import type { ExecutionContext } from '@nestjs/common';
import { Reflector } from '@nestjs/core';
import { JwtService } from '@nestjs/jwt';
import { I18nService } from 'nestjs-i18n';
import { Test, TestingModule } from '@nestjs/testing';
import { GqlExecutionContext } from '@nestjs/graphql';
import { RoleGuard } from './role.guard';
import { Permission, Role } from '../constants/enum.constant';

jest.mock('@nestjs/graphql', () => ({
  __esModule: true,
  ...jest.requireActual('@nestjs/graphql'),
  GqlExecutionContext: { create: jest.fn() },
}));

class HandlerClass {}

describe('RoleGuard', () => {
  let module: TestingModule;
  let guard: RoleGuard;
  let i18n: { t: jest.Mock };
  let jwtService: { verifyAsync: jest.Mock };
  let reflector: { getAllAndOverride: jest.Mock };
  let userService: { findById: jest.Mock };
  let request: any;
  let requiredRoles: any;
  let requiredPermissions: any;

  const buildGuard = async (withUserService = true) => {
    const providers: any[] = [
      RoleGuard,
      { provide: I18nService, useValue: i18n },
      { provide: JwtService, useValue: jwtService },
      { provide: Reflector, useValue: reflector },
    ];
    if (withUserService) {
      providers.push({ provide: 'USER_SERVICE', useValue: userService });
    }
    module = await Test.createTestingModule({ providers }).compile();
    guard = module.get(RoleGuard);
  };

  const createContext = (): ExecutionContext =>
    ({
      getHandler: () => HandlerClass,
      getClass: () => HandlerClass,
      getType: () => 'graphql',
      getArgs: () => [{ req: request }],
    }) as unknown as ExecutionContext;

  const setHeader = (value?: string) => {
    request = { headers: value === undefined ? {} : { authorization: value } };
  };

  const canActivate = async () => {
    (GqlExecutionContext.create as jest.Mock).mockReturnValue({
      getContext: () => ({ req: request }),
    });
    return guard.canActivate(createContext());
  };

  beforeEach(async () => {
    i18n = { t: jest.fn((key: string) => key) };
    jwtService = { verifyAsync: jest.fn() };
    reflector = { getAllAndOverride: jest.fn() };
    userService = { findById: jest.fn() };
    requiredRoles = undefined;
    requiredPermissions = undefined;
    reflector.getAllAndOverride.mockImplementation((key: string) =>
      key === 'roles' ? requiredRoles : requiredPermissions,
    );
    setHeader();
    await buildGuard();
  });

  afterEach(async () => {
    await module?.close();
  });

  it('throws UnauthorizedException when authorization header is missing', async () => {
    await expect(canActivate()).rejects.toThrow(UnauthorizedException);
    await expect(canActivate()).rejects.toThrow('user.NO_TOKEN');
    expect(i18n.t).toHaveBeenCalledWith('user.NO_TOKEN');
    expect(jwtService.verifyAsync).not.toHaveBeenCalled();
  });

  it('throws UnauthorizedException for malformed authorization header', async () => {
    setHeader('Basic abc.def');
    await expect(canActivate()).rejects.toThrow('user.NO_TOKEN');
    expect(jwtService.verifyAsync).not.toHaveBeenCalled();
  });

  it('throws UnauthorizedException when bearer token is absent', async () => {
    setHeader('Bearer');
    await expect(canActivate()).rejects.toThrow('user.NO_TOKEN');
    expect(jwtService.verifyAsync).not.toHaveBeenCalled();
  });

  it('throws UnauthorizedException when token verification fails', async () => {
    setHeader('Bearer bad-token');
    jwtService.verifyAsync.mockRejectedValue(new Error('invalid signature'));

    await expect(canActivate()).rejects.toThrow(UnauthorizedException);
    await expect(canActivate()).rejects.toThrow('user.INVALID_TOKEN');
    expect(jwtService.verifyAsync).toHaveBeenCalledWith(
      'bad-token',
      expect.objectContaining({ secret: process.env.JWT_SECRET }),
    );
    expect(i18n.t).toHaveBeenCalledWith('user.INVALID_TOKEN');
  });

  it('throws UnauthorizedException when payload has no sub or id', async () => {
    setHeader('Bearer token');
    jwtService.verifyAsync.mockResolvedValue({ role: Role.ADMIN });

    await expect(canActivate()).rejects.toThrow('user.INVALID_TOKEN');
    expect(userService.findById).not.toHaveBeenCalled();
  });

  it('returns true and populates request.user when role comes from token', async () => {
    setHeader('Bearer token');
    requiredRoles = [Role.ADMIN];
    requiredPermissions = [Permission.VIEW_USER, Permission.DELETE_USER];
    jwtService.verifyAsync.mockResolvedValue({
      sub: 'user-1',
      role: Role.ADMIN,
      email: 'admin@x.io',
      sessionId: 'session-1',
    });

    expect(await canActivate()).toBe(true);
    expect(userService.findById).not.toHaveBeenCalled();
    expect(request.user).toEqual({
      id: 'user-1',
      email: 'admin@x.io',
      role: Role.ADMIN,
      permissions: expect.arrayContaining([Permission.VIEW_USER, Permission.DELETE_USER]),
      sessionId: 'session-1',
    });
    expect(request.user.permissions).toContain(Permission.VIEW_USER);
  });

  it('falls back to payload.id when sub is missing', async () => {
    setHeader('Bearer token');
    jwtService.verifyAsync.mockResolvedValue({ id: 'legacy-9', role: Role.USER });

    expect(await canActivate()).toBe(true);
    expect(request.user.id).toBe('legacy-9');
    expect(request.user.role).toBe(Role.USER);
  });

  it('fetches role from user service when token has no role', async () => {
    setHeader('Bearer token');
    requiredRoles = [Role.ADMIN];
    jwtService.verifyAsync.mockResolvedValue({ sub: 'user-2' });
    userService.findById.mockResolvedValue({ data: { role: Role.ADMIN } });

    expect(await canActivate()).toBe(true);
    expect(userService.findById).toHaveBeenCalledWith('user-2');
    expect(request.user.role).toBe(Role.ADMIN);
  });

  it('accepts user service response without data wrapper', async () => {
    setHeader('Bearer token');
    requiredRoles = [Role.DRIVER];
    jwtService.verifyAsync.mockResolvedValue({ sub: 'driver-1' });
    userService.findById.mockResolvedValue({ role: Role.DRIVER });

    expect(await canActivate()).toBe(true);
    expect(request.user.role).toBe(Role.DRIVER);
  });

  it('throws when user service fails and role cannot be resolved', async () => {
    setHeader('Bearer token');
    requiredRoles = [Role.ADMIN];
    jwtService.verifyAsync.mockResolvedValue({ sub: 'user-3' });
    userService.findById.mockRejectedValue(new Error('user service down'));

    await expect(canActivate()).rejects.toThrow(UnauthorizedException);
    await expect(canActivate()).rejects.toThrow('user.INSUFFICIENT_PERMISSIONS');
    expect(i18n.t).toHaveBeenCalledWith('user.INSUFFICIENT_PERMISSIONS');
  });

  it('passes when user service fails but no roles or permissions are required', async () => {
    setHeader('Bearer token');
    jwtService.verifyAsync.mockResolvedValue({ sub: 'user-4' });
    userService.findById.mockRejectedValue(new Error('boom'));

    expect(await canActivate()).toBe(true);
    expect(request.user.permissions).toEqual([]);
  });

  it('does not query user service when it is not injected', async () => {
    await buildGuard(false);
    setHeader('Bearer token');
    requiredRoles = [Role.ADMIN];
    jwtService.verifyAsync.mockResolvedValue({ sub: 'user-5' });

    await expect(canActivate()).rejects.toThrow('user.INSUFFICIENT_PERMISSIONS');
    expect(userService.findById).not.toHaveBeenCalled();
  });

  it('throws on role mismatch', async () => {
    setHeader('Bearer token');
    requiredRoles = [Role.ADMIN];
    jwtService.verifyAsync.mockResolvedValue({ sub: 'user-6', role: Role.USER });

    await expect(canActivate()).rejects.toThrow('user.INSUFFICIENT_PERMISSIONS');
    expect(request.user).toBeUndefined();
  });

  it('throws on permission mismatch', async () => {
    setHeader('Bearer token');
    requiredPermissions = [Permission.DELETE_USER];
    jwtService.verifyAsync.mockResolvedValue({ sub: 'user-7', role: Role.USER });

    await expect(canActivate()).rejects.toThrow('user.INSUFFICIENT_PERMISSIONS');
  });

  it('returns true when role and permissions match', async () => {
    setHeader('Bearer token');
    requiredRoles = [Role.DRIVER];
    requiredPermissions = [Permission.UPDATE_DELIVERY_STATUS, Permission.VIEW_DELIVERY];
    jwtService.verifyAsync.mockResolvedValue({ sub: 'driver-2', role: Role.DRIVER });

    expect(await canActivate()).toBe(true);
    expect(request.user.role).toBe(Role.DRIVER);
    expect(request.user.permissions).toContain(Permission.UPDATE_DELIVERY_STATUS);
  });

  it('returns true when reflector exposes no roles or permissions', async () => {
    setHeader('Bearer token');
    jwtService.verifyAsync.mockResolvedValue({ sub: 'user-8', role: Role.USER });

    expect(await canActivate()).toBe(true);
    expect(reflector.getAllAndOverride).toHaveBeenCalledWith('roles', [
      HandlerClass,
      HandlerClass,
    ]);
    expect(reflector.getAllAndOverride).toHaveBeenCalledWith('permissions', [
      HandlerClass,
      HandlerClass,
    ]);
  });

  it('returns true for empty required lists even when token role is unknown', async () => {
    setHeader('Bearer token');
    requiredRoles = [];
    requiredPermissions = [];
    jwtService.verifyAsync.mockResolvedValue({ sub: 'user-9', role: 'ghost' });

    expect(await canActivate()).toBe(true);
    expect(request.user.permissions).toEqual([]);
  });
});
