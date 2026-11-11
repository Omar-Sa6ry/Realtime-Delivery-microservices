import { DataSource, QueryRunner } from 'typeorm';

export interface MockQueryRunner extends jest.Mocked<QueryRunner> {
  manager: {
    create: jest.Mock;
    save: jest.Mock;
  };
}

export interface MockDataSource extends jest.Mocked<DataSource> {
  createQueryRunner: jest.Mock<() => MockQueryRunner>;
}

export function createMockQueryRunner(
  options: { saveError?: Error } = {},
): MockQueryRunner {
  const manager = {
    create: jest
      .fn()
      .mockImplementation((_entity: any, data?: Record<string, unknown>) => ({
        ...data,
        id: data?.['id'] ?? `generated-id-${manager.create.mock.calls.length}`,
      })),
    save: jest.fn().mockImplementation(async (entity: any) => {
      if (options.saveError) {
        throw options.saveError;
      }
      return entity;
    }),
  };

  return {
    connect: jest.fn().mockResolvedValue(undefined),
    startTransaction: jest.fn().mockResolvedValue(undefined),
    commitTransaction: jest.fn().mockResolvedValue(undefined),
    rollbackTransaction: jest.fn().mockResolvedValue(undefined),
    release: jest.fn().mockResolvedValue(undefined),
    isTransactionActive: false,
    isInTransaction: false,
    query: jest.fn().mockResolvedValue([]),
    manager,
  } as unknown as MockQueryRunner;
}

export function createMockDataSource(
  queryRunner: MockQueryRunner,
): MockDataSource {
  return {
    createQueryRunner: jest.fn(() => queryRunner),
  } as unknown as MockDataSource;
}
