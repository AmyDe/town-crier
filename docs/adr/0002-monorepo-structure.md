# ADR 0002: Monorepo Directory Structure

## Status
Accepted

## Context
The project requires a structure to house multiple distinct components (iOS app, .NET API, infrastructure) within a single repository. The team also mandates a clear separation of source code and tests.

## Decision
1.  **Adopt a flat root-level structure** (`/mobile`, `/api`, `/infra`) rather than nesting under an `apps/` directory.
2.  **Enforce a strict separation of source and test code** within each component, following platform-specific industry standards.
3.  **Implement a Layered Architecture** within each component as defined below:

### API (.NET 10) Internal Structure
Located in `/api`, following a **Hexagonal (Ports & Adapters)** pattern:
- `src/town-crier.domain/`: **Domain Layer (Core)** - Pure business logic, Entities, and Value Objects. No external dependencies.
- `src/town-crier.application/`: **Application Layer (Ports)** - Use Cases, Command/Query Handlers, and Port interfaces. Lightweight orchestrators that delegate to the Domain.
- `src/town-crier.infrastructure/`: **Infrastructure Layer (Adapters)** - Implementations of Ports (Repositories, API Clients), and Controllers.
- `src/town-crier.web/`: **Web/Entry Point** - Program.cs, configuration, and Native AOT bootstrap.
- `tests/town-crier.application.tests/`: **Primary Testing Unit** - TUnit tests focusing on Handlers and business behavior.

*Note: While directory names and project files use `town-crier.*` (lowercase), internal C# namespaces should follow standard .NET PascalCase conventions (e.g., `TownCrier.Domain`).*

### Mobile Internal Structure
Located in `/mobile`, with platform-specific subdirectories to allow for future expansion:

#### iOS (`/mobile/ios`)
Utilizing **Clean Architecture (MVVM-C)** and **Swift Package Manager (SPM)** for modularization:
- `town-crier-app/`: Main application target, entry point, and global Coordinators.
- `packages/`: Shared and feature-specific SPM modules.
    - `town-crier-domain/`: Entities, Value Objects, and Repository Protocols (Pure Swift).
    - `town-crier-data/`: API Clients and persistence (SwiftData) implementations.
    - `town-crier-presentation/`: ViewModels, SwiftUI Views, and feature Coordinators.
- `town-crier-tests/`: XCTest suite for integration and UI tests.

### Infrastructure (Pulumi) Internal Structure
Located in `/infra`, utilizing **.NET 10 (C#)**:
- `src/town-crier.infra/`: Pulumi stacks and resource definitions.
- `tests/town-crier.infra.tests/`: Unit tests for infrastructure policies and configurations.


## Rationale
- **Flat Structure:** Minimizes nesting depth and simplifies navigation for a project with a known, finite set of applications.
- **Source/Test Separation:** Ensures testability is a first-class concern and keeps production code clean.
- **Hexagonal Architecture (.NET):** Decouples business logic from external concerns (DB, API), ensuring the Domain remains pure and testable.
- **Clean Architecture (iOS):** Promotes modularity and separation of concerns via SPM, allowing for independent development and testing of features.
- **Mobile Pathing:** Nesting platforms within `/mobile` (e.g., `/mobile/ios`) anticipates multi-platform support while maintaining a clean root.
- **Industry Standards:**
    - **.NET 10 (API & Infra):** Will use the standard `src/` and `tests/` folder layout.
    - **iOS (Mobile):** Will use standard Xcode target separation (e.g., `App/` for source, `AppTests/` for unit tests), which maps to the "src/test" concept while remaining tool-friendly.
    - **Containerization:** The API will include a `Dockerfile` to support deployment to Azure Container Apps.

## Consequences
- New top-level directories will be created for each major component:
    - `mobile/ios/`: Source code for the iOS application.
    - `api/`: Source code for the .NET 10 backend API (including Dockerfile).
    - `infra/`: Pulumi infrastructure code (C#/.NET 10).
- Project templates (sln, xcodeproj) must be configured to respect these internal structures.
