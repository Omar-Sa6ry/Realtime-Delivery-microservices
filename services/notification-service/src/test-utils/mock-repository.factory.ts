import { Repository, ObjectLiteral } from 'typeorm';

export type MockRepository<T extends ObjectLiteral = any> = jest.Mocked<
  Pick<
    Repository<T>,
    | 'find'
    | 'findOne'
    | 'findOneBy'
    | 'findAndCount'
    | 'create'
    | 'save'
    | 'update'
    | 'delete'
    | 'count'
    | 'remove'
    | 'insert'
  >
> &
  Partial<Repository<T>>;

export function createMockRepository<
  T extends ObjectLiteral = any,
>(): MockRepository<T> {
  return {
    find: jest.fn().mockResolvedValue([]),
    findOne: jest.fn().mockResolvedValue(null),
    findOneBy: jest.fn().mockResolvedValue(null),
    findAndCount: jest.fn().mockResolvedValue([[], 0]),
    create: jest.fn().mockImplementation((_entityOrOptions?: any) => ({
      ...(typeof _entityOrOptions === 'object' ? _entityOrOptions : {}),
    })),
    save: jest.fn().mockImplementation(async (entity: any) => entity),
    update: jest.fn().mockResolvedValue({ affected: 1, raw: {} }),
    delete: jest.fn().mockResolvedValue({ affected: 1, raw: [] }),
    count: jest.fn().mockResolvedValue(0),
    remove: jest.fn().mockImplementation(async (entity: any) => entity),
    insert: jest.fn().mockResolvedValue({ identifiers: [], raw: {} }),
  } as MockRepository<T>;
}
