import { Queue } from 'bullmq';

export type MockQueue = jest.Mocked<
  Pick<Queue, 'add' | 'upsertJobScheduler' | 'getJob' | 'removeJobScheduler'>
>;

export function createMockQueue(): MockQueue {
  return {
    add: jest.fn().mockResolvedValue({ id: 'job-1' }),
    upsertJobScheduler: jest.fn().mockResolvedValue(undefined),
    getJob: jest.fn().mockResolvedValue(null),
    removeJobScheduler: jest.fn().mockResolvedValue(true),
  } as unknown as MockQueue;
}

export function createMockQueuesMap<K extends string>(
  keys: K[],
): Record<K, MockQueue> {
  const map = {} as Record<K, MockQueue>;
  for (const key of keys) {
    map[key] = createMockQueue();
  }
  return map;
}
