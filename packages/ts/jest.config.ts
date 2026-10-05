import type { Config } from 'jest';

const config: Config = {
  moduleFileExtensions: ['js', 'json', 'ts'],
  rootDir: 'src',
  testRegex: '.*\\.spec\\.ts$',
  transform: {
    '^.+\\.(t|j)s$': [
      'ts-jest',
      {
        diagnostics: false,
      },
    ],
  },
  collectCoverageFrom: [
    '**/*.(t|j)s',
    '!**/index.ts',
    '!**/*.module.ts',
    '!**/*.interface.ts',
    '!**/*.dto.ts',
    '!**/*.events.ts',
    '!**/*.constant.ts',
    '!**/*.constants.ts',
    '!**/*.topics.ts',
    '!**/events/envelope.ts',
  ],
  coverageDirectory: '../coverage',
  coverageReporters: ['text', 'html', 'lcov'],
  testEnvironment: 'node',
  workerThreads: true,
  coverageThreshold: {
    global: {
      branches: 85,
      functions: 90,
      lines: 90,
      statements: 90,
    },
  },
};

export default config;
