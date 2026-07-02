# Architecture: DDD & MVVM-C (reference)

Read when scaffolding a package or feature, or shaping an entity, value object, ViewModel, Coordinator, or View. The core (`SKILL.md`) states the rules; this file is the project-structure tree, the DDD/MVVM-C rationale, and the full examples.

## Overview

This skill provides guidelines for Swift/iOS development in the Town Crier app, prioritizing **Protocol-Oriented Programming**, **Clean Architecture (MVVM-C)**, and **Test-Driven Development (TDD)** using XCTest.

The core idea behind these patterns is that the domain logic — the business rules about planning applications, subscriptions, and notifications — should be completely independent of UIKit, SwiftUI, SwiftData, or any Apple framework. This makes the domain testable, portable, and easy to reason about. Everything else (UI, persistence, networking) is an implementation detail that plugs in from the outside.

## Project Structure

The iOS app lives in `/mobile/ios` using SPM for modularization:

```
/mobile/ios/
├── town-crier-app/                    # Main app target, entry point, global Coordinators
│   └── Sources/
│       ├── TownCrierApp.swift         # @main entry, composition root
│       └── AppCoordinator.swift       # Root navigation coordinator
├── packages/
│   ├── town-crier-domain/             # Pure Swift — no Apple framework imports
│   │   └── Sources/
│   │       ├── Entities/              # PlanningApplication, AlertSubscription, etc.
│   │       ├── ValueObjects/          # Postcode, LocalAuthority, ApplicationReference
│   │       └── Protocols/             # Repository protocols (ports)
│   ├── town-crier-data/               # API clients, SwiftData, repository implementations
│   │   └── Sources/
│   │       ├── API/                   # URLSession-based API client
│   │       ├── Persistence/           # SwiftData models and context
│   │       └── Repositories/          # Concrete repository implementations (adapters)
│   └── town-crier-presentation/       # ViewModels, Views, feature Coordinators
│       └── Sources/
│           ├── Features/
│           │   ├── AuthorityList/     # AuthorityListViewModel, AuthorityListView
│           │   ├── ApplicationFeed/   # ApplicationFeedViewModel, ApplicationFeedView
│           │   └── ApplicationDetail/ # ApplicationDetailViewModel, ApplicationDetailView
│           └── Coordinators/          # Feature-level navigation coordinators
└── town-crier-tests/                  # XCTest suite
    └── Sources/
        ├── Spies/                     # Protocol-conforming spy implementations
        ├── Fixtures/                  # Static extension test data
        └── Features/                  # Tests organised by feature
```

Directory and package names use `town-crier-*` (lowercase, hyphenated). Swift types use PascalCase.

## Domain-Driven Design (DDD)

Value types make equality checks trivial, prevent accidental shared-mutation bugs, and are optimised by the Swift compiler for stack allocation. Keeping the domain layer framework-free means you can test it without simulators or mocks of Apple APIs.

- **Value Types First:** Prefer `struct` over `class` for all data models. Immutability is the default.
- **Domain Purity:** The Domain package must not import `UIKit`, `SwiftUI`, `SwiftData`, or any third-party framework. `import Foundation` is acceptable when you need stdlib-adjacent types (`Date`, `UUID`, `URL`, regex), but avoid pulling in Foundation-specific behaviour (e.g., `URLSession`, `JSONEncoder`, `FileManager`) — those belong in the Data layer.
- **Rich Models:** Encapsulate business logic as methods and computed properties on the model, not in ViewModels or external services. Use extensions to organise computed logic.
- **Error Handling:** Use typed `Result<T, DomainError>` or `throws` with defined `Error` enums. Never return optional `nil` to suppress errors — the caller deserves to know *why* something failed.

**Example — Rich Domain Entity:**
```swift
struct PlanningApplication {
    let id: PlanningApplicationId
    let reference: ApplicationReference
    let authority: LocalAuthority
    private(set) var status: ApplicationStatus
    let receivedDate: Date
    let description: String
    let address: String

    mutating func markAsDecided(_ decision: Decision, on decisionDate: Date) throws {
        guard status == .underReview else {
            throw DomainError.invalidStatusTransition(
                from: status, to: decision == .approved ? .approved : .refused
            )
        }
        status = decision == .approved ? .approved : .refused
    }
}
```

**Example — Value Object with validation:**
```swift
struct Postcode: Equatable, Hashable {
    let value: String

    init(_ raw: String) throws {
        let trimmed = raw.trimmingCharacters(in: .whitespaces).uppercased()
        guard Self.isValid(trimmed) else {
            throw DomainError.invalidPostcode(raw)
        }
        value = trimmed
    }

    private static func isValid(_ postcode: String) -> Bool {
        let pattern = #"^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$"#
        return postcode.range(of: pattern, options: .regularExpression) != nil
    }
}
```

## Architecture Style (Clean / MVVM-C)

Clean Architecture separates concerns so that business rules don't depend on UI choices, and UI choices don't depend on persistence choices. MVVM-C adds Coordinators to own navigation flow, keeping Views genuinely passive.

- **Dependency Rule:** Dependencies flow **inward**. Views depend on ViewModels. ViewModels depend on Domain protocols (repository ports, entities). Neither Views nor ViewModels know about concrete data-layer implementations.
- **Coordinators:** Navigation logic belongs in **Coordinators**, never in Views. Views should not know about other Views — they publish intents (e.g., "user tapped application") and the Coordinator decides what happens next. This makes navigation testable and prevents deep coupling between screens.
- **Composition Root:** Wire all dependencies at the app entry point (`TownCrierApp.swift`). Use manual dependency injection or a lightweight container. Do not propagate core services via `@EnvironmentObject` — it hides dependencies and makes testing harder.

**Example — ViewModel (Presentation layer):**
```swift
@MainActor
final class ApplicationFeedViewModel: ObservableObject {
    @Published private(set) var applications: [PlanningApplication] = []
    @Published private(set) var isLoading = false
    @Published private(set) var error: DomainError?

    private let repository: PlanningApplicationRepository
    var onApplicationSelected: ((PlanningApplicationId) -> Void)?

    init(repository: PlanningApplicationRepository) {
        self.repository = repository
    }

    func loadApplications(for authority: LocalAuthority) async {
        isLoading = true
        error = nil
        do {
            applications = try await repository.fetchApplications(for: authority)
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .unexpected(error)
        }
        isLoading = false
    }

    func selectApplication(_ id: PlanningApplicationId) {
        onApplicationSelected?(id)
    }
}
```

**Example — Coordinator:**
```swift
@MainActor
final class ApplicationFeedCoordinator: ObservableObject {
    @Published var detailApplication: PlanningApplication?

    private let repository: PlanningApplicationRepository

    init(repository: PlanningApplicationRepository) {
        self.repository = repository
    }

    func makeApplicationFeedViewModel() -> ApplicationFeedViewModel {
        let viewModel = ApplicationFeedViewModel(repository: repository)
        viewModel.onApplicationSelected = { [weak self] id in
            Task { await self?.showDetail(for: id) }
        }
        return viewModel
    }

    private func showDetail(for id: PlanningApplicationId) async {
        detailApplication = try? await repository.fetchApplication(by: id)
    }
}
```

**Example — Passive View:**
```swift
struct ApplicationFeedView: View {
    @StateObject private var viewModel: ApplicationFeedViewModel

    init(viewModel: ApplicationFeedViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        List(viewModel.applications, id: \.id) { application in
            ApplicationRow(application: application)
                .onTapGesture { viewModel.selectApplication(application.id) }
        }
        .overlay {
            if viewModel.isLoading { ProgressView() }
        }
    }
}
```
