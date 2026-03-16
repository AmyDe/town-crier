# 0005. Dockerised Test Environment with One-Click Execution

Date: 2026-03-16

## Status

Accepted

## Context

Integration tests require backing services (mock PlanIt API per ADR 0004, Cosmos DB emulator, and the Town Crier API itself). Manual setup of these services is error-prone and creates onboarding friction. We want a new developer to be able to clone the repo, open it in VSCode, and click "Run All Tests" — with all tests, including integration tests, passing on the first attempt.

## Decision

We will use **Docker Compose** to fully containerise the test environment:

1. **`docker-compose.yml`** at the repository root orchestrates all services needed for integration tests:
   - Mock PlanIt API (ADR 0004)
   - Town Crier API (built from `/api`)
   - Azure Cosmos DB emulator (or compatible substitute)

2. **Automatic lifecycle management** — The .NET integration test fixtures will start the Docker Compose environment automatically before tests run and tear it down afterwards. No manual `docker compose up` required.

3. **VSCode compatibility** — Since `dotnet test` triggers the Docker Compose lifecycle, VSCode's built-in "Run All Tests" button works without additional configuration. Docker Desktop is the only prerequisite.

4. **CI parity** — The same Docker Compose setup runs in GitHub Actions, ensuring local and CI test environments are identical.

## Consequences

- **Zero-setup onboarding** — Clone, open, run tests. Docker Desktop is the only prerequisite beyond the .NET SDK.
- **Environment consistency** — Local dev, CI, and new contributor setups all use the same containerised services.
- **Docker dependency** — All developers and CI runners must have Docker Desktop (or equivalent) installed.
- **First-run latency** — Initial `docker compose up` pulls images and builds containers, adding time to the first test run. Subsequent runs reuse cached images/containers.
- **Resource usage** — Running Cosmos DB emulator and other containers requires reasonable RAM/CPU. Developers need a machine that can handle Docker workloads alongside their IDE.
