# 0016. Test Infrastructure Strategy

Date: 2026-03-31

## Status

Accepted — supersedes [0005](0005-dockerised-test-environment.md)

## Context

ADR 0005 proposed a Docker Compose test environment with a Cosmos DB emulator, mock PlanIt API container, and the Town Crier API — all orchestrated automatically from .NET test fixtures so that `dotnet test` "just works" from VSCode.

In practice, this approach was not implemented. Instead, the test infrastructure evolved in two directions:

1. **Unit tests** use in-memory fakes — every Cosmos DB repository has a matching `InMemory*Repository` implementation in the infrastructure layer, and external HTTP services use `FakeHttpMessageHandler` test doubles. The `TestWebApplicationFactory` replaces all Cosmos repositories with in-memory implementations and overrides JWT authentication with a test signing key. No Docker containers are needed.

2. **Integration tests** deploy to a real staging environment. The PR gate workflow (`pr-gate.yml`) builds a Docker image, deploys it to the dev Azure Container App as a staging revision with 0% traffic, runs integration tests against the staging URL with real Auth0 credentials, and promotes the revision on success (see [ADR 0015](0015-cicd-pipeline-and-deployment-strategy.md)).

The Docker Compose + emulator approach was abandoned because:

- The Cosmos DB emulator is Linux-only in container form and has limited feature parity with the serverless service (no spatial indexes, no change feed processor in the emulator at the time of evaluation).
- In-memory fakes are faster, deterministic, and run without Docker Desktop — reducing CI costs and developer machine requirements.
- Staging-based integration tests validate the real deployment pipeline, authentication flow, and Azure resource configuration — things a local emulator cannot cover.

## Decision

The test infrastructure uses a **two-tier strategy**:

### Tier 1: Unit and handler tests (in-memory, no Docker)

- All repository ports have dual implementations: production Cosmos DB adapters and `InMemory*` test doubles.
- External HTTP services (PlanIt, Gov.uk Planning Data, Postcodes.io) use `Fake*Handler` message handler stubs seeded with fixed response data.
- `TestWebApplicationFactory` wires in-memory implementations for all infrastructure dependencies.
- Tests run with `dotnet test` — no Docker, no containers, no network access.
- One-click execution in VSCode and GitHub Actions.

### Tier 2: Integration tests (staging deployment, real Azure services)

- The PR gate deploys the API to the dev Container App as a staging revision.
- Integration tests run against the staging URL, authenticating with Auth0 test credentials stored as GitHub secrets.
- Tests validate end-to-end flows: authentication, profile creation, watch zone management, application retrieval.
- On success, the staging revision is promoted to serve traffic. On failure, it is deactivated and cleaned up.

### Web and iOS testing

- Web tests use Vitest with jsdom, Testing Library, and spy/stub implementations of port interfaces. No Docker.
- iOS tests use Swift Testing with spy/stub protocol implementations. No Docker.

## Consequences

- **No Docker Desktop requirement for developers.** All unit tests run without containers. This lowers the onboarding bar and speeds up the feedback loop.
- **Real-environment integration coverage.** Staging-based tests catch deployment, networking, and authentication issues that emulators cannot reproduce.
- **CI cost trade-off.** Staging deployments add ~2 minutes to the PR gate but avoid maintaining a Cosmos DB emulator container and Docker Compose orchestration.
- **Test data isolation.** In-memory fakes provide perfect isolation between test runs. Staging tests must manage their own test data lifecycle (create/teardown per test).
- **No offline integration testing.** Developers cannot run integration tests locally without network access to Azure. This is acceptable because unit tests cover business logic, and integration tests are a CI-only concern.
