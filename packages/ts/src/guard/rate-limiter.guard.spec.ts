import * as validation from '@bts-soft/validation';
import {
  RateLimiter,
  RateLimit,
  RateLimiterAlgorithm,
  RedisStore,
} from './rate-limiter.guard';

jest.mock('@bts-soft/validation', () => ({
  __esModule: true,
  RateLimiter: jest.fn(),
  RateLimit: jest.fn(),
  RateLimiterAlgorithm: {
    TOKEN_BUCKET: 'TOKEN_BUCKET',
    LEAKING_BUCKET: 'LEAKING_BUCKET',
    FIXED_WINDOW_COUNTER: 'FIXED_WINDOW_COUNTER',
    SLIDING_WINDOW_LOG: 'SLIDING_WINDOW_LOG',
    SLIDING_WINDOW_COUNTER: 'SLIDING_WINDOW_COUNTER',
  },
  RedisStore: jest.fn(),
}));

describe('rate-limiter.guard', () => {
  it('re-exports RateLimiter', () => {
    expect(RateLimiter).toBe(validation.RateLimiter);
    expect(RateLimiter).toBeDefined();
  });

  it('re-exports RateLimit', () => {
    expect(RateLimit).toBe(validation.RateLimit);
    expect(RateLimit).toBeDefined();
  });

  it('re-exports RedisStore', () => {
    expect(RedisStore).toBe(validation.RedisStore);
    expect(RedisStore).toBeDefined();
  });

  it('re-exports RateLimiterAlgorithm with expected members', () => {
    expect(RateLimiterAlgorithm).toBe(validation.RateLimiterAlgorithm);
    expect(RateLimiterAlgorithm.TOKEN_BUCKET).toBe('TOKEN_BUCKET');
    expect(RateLimiterAlgorithm.LEAKING_BUCKET).toBe('LEAKING_BUCKET');
    expect(RateLimiterAlgorithm.FIXED_WINDOW_COUNTER).toBe('FIXED_WINDOW_COUNTER');
    expect(RateLimiterAlgorithm.SLIDING_WINDOW_LOG).toBe('SLIDING_WINDOW_LOG');
    expect(RateLimiterAlgorithm.SLIDING_WINDOW_COUNTER).toBe('SLIDING_WINDOW_COUNTER');
  });
});
