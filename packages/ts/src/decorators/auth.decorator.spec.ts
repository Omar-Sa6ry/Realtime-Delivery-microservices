import { Auth } from './auth.decorator';
import { RoleGuard } from '../guard/role.guard';

describe('Auth decorator', () => {
  const makeClass = () => {
    class Target {
      handle() {
        return 'handled';
      }
    }
    return Target;
  };

  const applyToMethod = (permissions?: string[]) => {
    const Target = makeClass();
    const descriptor = Object.getOwnPropertyDescriptor(Target.prototype, 'handle')!;
    const decorator = permissions === undefined ? Auth() : Auth(permissions);
    decorator(Target.prototype, 'handle', descriptor);
    return { Target, descriptor };
  };

  it('registers RoleGuard and default empty permissions on handler', () => {
    const { descriptor } = applyToMethod();

    expect(Reflect.getMetadata('__guards__', descriptor.value)).toEqual([RoleGuard]);
    expect(Reflect.getMetadata('permissions', descriptor.value)).toEqual([]);
    expect(descriptor.value.call(Object.create(null))).toBe('handled');
  });

  it('registers provided permissions on handler', () => {
    const { descriptor } = applyToMethod(['view_user', 'delete_user']);

    expect(Reflect.getMetadata('permissions', descriptor.value)).toEqual([
      'view_user',
      'delete_user',
    ]);
    expect(Reflect.getMetadata('__guards__', descriptor.value)).toEqual([RoleGuard]);
  });

  it('registers RoleGuard and permissions when applied to a class', () => {
    const Target = makeClass();

    Auth(['manage_users'])(Target);

    expect(Reflect.getMetadata('__guards__', Target)).toEqual([RoleGuard]);
    expect(Reflect.getMetadata('permissions', Target)).toEqual(['manage_users']);
  });

  it('does not leak metadata between targets', () => {
    const { descriptor } = applyToMethod(['only_here']);
    const Other = makeClass();

    expect(Reflect.getMetadata('permissions', descriptor.value)).toEqual(['only_here']);
    expect(Reflect.getMetadata('permissions', Other)).toBeUndefined();
    expect(Reflect.getMetadata('__guards__', Other)).toBeUndefined();
  });
});
