---
name: ios-coding-standards
description: "MUST consult before writing ANY Swift code. Enforces iOS coding standards for /mobile/ios: MVVM-C architecture, Swift Concurrency, XCTest patterns, SwiftData, and SwiftLint rules. Trigger this skill whenever the user asks you to: write, create, scaffold, or generate any Swift file or iOS code; write or update XCTest tests, spies, fakes, or fixtures; create or modify a ViewModel, View, Coordinator, entity, value object, or repository; set up or configure SwiftLint, swift-format, or SPM packages; refactor Swift code (e.g. replace DispatchQueue with async/await, extract logic from Views to ViewModels, add dependency injection); review Swift code or PRs for standards compliance; work with SwiftData models or persistence layers; create domain models in town-crier-domain, town-crier-data, or town-crier-presentation packages. Even if the task seems simple, ALWAYS check this skill first when Swift or iOS is involved — it contains project-specific patterns (Coordinator callbacks, spy naming, fixture conventions, repository protocols) that differ from generic Swift. Do NOT use for C#/.NET, Pulumi, CI/CD, Dockerfiles, or general architecture questions."
---

# iOS Coding Standards

Protocol-oriented, Clean Architecture (MVVM-C), TDD for the Town Crier iOS app (`/mobile/ios`). The domain logic — planning applications, subscriptions, notifications — must be completely independent of UIKit, SwiftUI, SwiftData, or any Apple framework; UI, persistence, and networking plug in from the outside. Read this core first; pull the matching reference below when the bead touches that area.

## Architecture (always applies)

- **SPM package split under `/mobile/ios`, names `town-crier-*` (lowercase, hyphenated); Swift types PascalCase.** `town-crier-domain` = pure Swift (Entities, ValueObjects, repository-protocol ports) — no `UIKit`/`SwiftUI`/`SwiftData`/third-party imports (`import Foundation` for `Date`/`UUID`/`URL`/regex only; Foundation *behaviour* like `URLSession`/`JSONEncoder`/`FileManager` belongs in Data). `town-crier-data` = API clients, SwiftData, repository implementations (adapters). `town-crier-presentation` = ViewModels, Views, feature Coordinators. `town-crier-app` = `@main` entry + composition root.
- **Value types first** — prefer `struct` over `class`; immutability is the default. **Rich models:** encapsulate business logic as methods/computed properties on the model, not in ViewModels or external services.
- **MVVM-C, dependencies flow inward.** Views depend on ViewModels; ViewModels depend on Domain protocols (repository ports, entities); neither knows concrete data-layer types. Navigation lives in **Coordinators** — Views publish intents (e.g. `onApplicationSelected`), the Coordinator decides. Wire all dependencies at the composition root (`TownCrierApp.swift`) with manual DI, not `@EnvironmentObject`.
- **Swift Concurrency (`async`/`await`) exclusively** — no `DispatchQueue`/completion handlers/`Combine` for request/response work. All UI-bound ViewModel state is `@MainActor`.
- **Repository protocols are defined in the Domain package**; implementations live in Data and hide all SwiftData/API concerns. The app layer speaks domain entities, never persistence types. **Errors** are typed (`throws` with a defined `Error` enum, or `Result<T, DomainError>`) — never return optional `nil` to suppress an error.

## Test-double conventions (always applies)

- **XCTest only** — no BDD frameworks (Quick/Nimble). TDD: Red-Green-Refactor. ViewModels and Use Cases are the primary test targets; domain entities with business rules warrant direct unit tests.
- **Hand-written protocol-oriented spies** conforming to repository protocols, recording calls and returning preconfigured results — no reflection-based mocking libraries. Spies named `Spy<Protocol>` (e.g. `SpyPlanningApplicationRepository`), with capture arrays (`<method>Calls`) and stubbed `<method>Result` values.
- **Fixtures** are static extension properties (e.g. `PlanningApplication.pendingReview`). Prefer `init` with default parameters; use Builder classes only when construction is genuinely complex.
- **Async testing** uses `await` directly — no legacy `XCTestExpectation`.

## Forbidden

- Importing `UIKit`/`SwiftUI`/`SwiftData`/third-party frameworks in the Domain package.
- Foundation-specific behaviour (`URLSession`, `JSONEncoder`, `FileManager`) in the Domain package (it belongs in Data).
- Navigation logic in Views; a View knowing about other Views.
- `@EnvironmentObject` for propagating core services.
- Returning optional `nil` to suppress an error.
- `DispatchQueue.main.async` (except wrapping a legacy API with no async alternative).
- Completion handlers for async logic (use `async throws`).
- `Combine` for one-off request/response async (reactive streams only).
- BDD frameworks (Quick/Nimble); reflection-based mocking libraries.
- Legacy `XCTestExpectation` for modern async code.
- Force unwrap `!` outside XCTest assertions; SwiftLint treats force cast / force try / force unwrap as errors.
- Non-`final` classes without an explicitly designed, documented inheritance need (`final` by default).
- `I` prefix on protocol names (a C# convention).
- The app layer speaking persistence-specific types instead of domain entities.

## References (load on demand)

- `references/architecture.md` — read when scaffolding a package/feature, or shaping an entity, value object, ViewModel, Coordinator, or View (project-structure tree + DDD/MVVM-C rationale and examples).
- `references/concurrency.md` — read when writing async code, wrapping a legacy callback API, or annotating ViewModel state with `@MainActor`.
- `references/data-access.md` — read when the bead touches persistence, SwiftData, repository implementations, or DTO↔domain mapping (SwiftData model + repository adapter examples).
- `references/testing.md` — read when writing any test, spy, fake, or fixture (repository-protocol / spy / fixture / ViewModel-test examples).
- `references/workflow-and-naming.md` — read when running lint/format/build, configuring SwiftLint or swift-format, or naming a type/protocol (verification commands + naming & best-practice detail).
