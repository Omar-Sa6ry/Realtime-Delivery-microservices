# Contributing Guidelines

Thank you for your interest in contributing to the Realtime Delivery Microservices platform.

## Development Workflow

1. Fork or branch from `main`.
2. Ensure you have the required prerequisites:
   - Go 1.22+
   - Node.js 20+
   - Docker and Docker Compose
   - Task / Make
3. Copy environment configuration:
   ```bash
   cp config/env/.env.example config/env/.env.development
   ```
4. Start local infrastructure dependencies:
   ```bash
   task infra-up
   ```

## Code Standards & Testing

- **Testing**: Ensure that all unit and integration tests pass before submitting changes:
  - Unit tests: `./scripts/test-unit.ps1`
  - Integration/E2E tests: `./scripts/test-e2e.ps1`
- **Linting**: Run Trunk meta-linter to check formatting and conventions:
  - `trunk check`
- **Architecture**:
  - Go microservices follow Hexagonal Architecture (Ports and Adapters).
  - TypeScript services follow NestJS modular domain design.
  - Inter-service RPC changes must be reflected in `protos/` definitions.

## Pull Request Process

- Provide a descriptive title and summary of changes.
- Reference any related issue numbers.
- Verify that CI workflows pass successfully.
