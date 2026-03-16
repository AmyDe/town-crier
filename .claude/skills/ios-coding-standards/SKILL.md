---
name: ios-coding-standards
description: "MUST consult before writing ANY Swift code. Enforces iOS coding standards for /mobile/ios: MVVM-C architecture, Swift Concurrency, XCTest patterns, SwiftData, and SwiftLint rules. Trigger this skill whenever the user asks you to: write, create, scaffold, or generate any Swift file or iOS code; write or update XCTest tests, spies, fakes, or fixtures; create or modify a ViewModel, View, Coordinator, entity, value object, or repository; set up or configure SwiftLint, swift-format, or SPM packages; refactor Swift code (e.g. replace DispatchQueue with async/await, extract logic from Views to ViewModels, add dependency injection); review Swift code or PRs for standards compliance; work with SwiftData models or persistence layers; create domain models in town-crier-domain, town-crier-data, or town-crier-presentation packages. Even if the task seems simple, ALWAYS check this skill first when Swift or iOS is involved — it contains project-specific patterns (Coordinator callbacks, spy naming, fixture conventions, repository protocols) that differ from generic Swift. Do NOT use for C#/.NET, Pulumi, CI/CD, Dockerfiles, or general architecture questions."
---

# iOS Coding Standards

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

## Core Mandates

### 1. Domain-Driven Design (DDD)

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

### 2. Architecture Style (Clean / MVVM-C)

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

### 3. Testing Strategy (TDD & XCTest)

Protocol-oriented mocking avoids reflection (which breaks with Swift's concurrency model and adds fragile coupling to internal types). Manual spies are explicit about what they capture, making tests easier to read and debug.

- **Framework:** XCTest only. No BDD frameworks (Quick/Nimble) — they add DSL overhead without meaningful benefit for this project's scale.
- **Unit of Work:** ViewModels and Use Cases are the primary test targets because they contain the orchestration logic. Domain entities with business rules also warrant direct unit tests.
- **Workflow:** Red-Green-Refactor. Write the test first, watch it fail, make it pass, then clean up.
- **Protocol-Oriented Spies:** Create manual `Spy` classes conforming to repository protocols. Spies record calls and return preconfigured results. Do not use reflection-based mocking libraries.
- **Fixtures:** Use static extension properties for test data (e.g., `PlanningApplication.pendingReview`). Swift `init` with default parameters is usually sufficient — only use Builder classes if construction is genuinely complex.
- **Async Testing:** Use `await` directly in tests. Do not use legacy `XCTestExpectation` for modern async code.

**Example — Repository Protocol (Domain layer port):**
```swift
protocol PlanningApplicationRepository {
    func fetchApplications(for authority: LocalAuthority) async throws -> [PlanningApplication]
    func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication
}
```

**Example — Spy (Test target):**
```swift
final class SpyPlanningApplicationRepository: PlanningApplicationRepository {
    private(set) var fetchApplicationsCalls: [LocalAuthority] = []
    var fetchApplicationsResult: Result<[PlanningApplication], Error> = .success([])

    func fetchApplications(for authority: LocalAuthority) async throws -> [PlanningApplication] {
        fetchApplicationsCalls.append(authority)
        return try fetchApplicationsResult.get()
    }

    private(set) var fetchApplicationCalls: [PlanningApplicationId] = []
    var fetchApplicationResult: Result<PlanningApplication, Error> = .success(.pendingReview)

    func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
        fetchApplicationCalls.append(id)
        return try fetchApplicationResult.get()
    }
}
```

**Example — Fixture (static extension):**
```swift
extension PlanningApplication {
    static let pendingReview = PlanningApplication(
        id: PlanningApplicationId("APP-001"),
        reference: ApplicationReference("2026/0042"),
        authority: .cambridge,
        status: .underReview,
        receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
        description: "Erection of two-storey rear extension",
        address: "12 Mill Road, Cambridge, CB1 2AD"
    )

    static let approved = PlanningApplication(
        id: PlanningApplicationId("APP-002"),
        reference: ApplicationReference("2026/0099"),
        authority: .cambridge,
        status: .approved,
        receivedDate: Date(timeIntervalSince1970: 1_700_100_000),
        description: "Change of use from retail to residential",
        address: "45 High Street, Cambridge, CB2 1LA"
    )
}
```

**Example — ViewModel Test:**
```swift
@MainActor
final class ApplicationFeedViewModelTests: XCTestCase {
    private var spy: SpyPlanningApplicationRepository!
    private var sut: ApplicationFeedViewModel!

    override func setUp() {
        spy = SpyPlanningApplicationRepository()
        sut = ApplicationFeedViewModel(repository: spy)
    }

    func test_loadApplications_populatesApplicationsOnSuccess() async {
        let expected = [PlanningApplication.pendingReview, .approved]
        spy.fetchApplicationsResult = .success(expected)

        await sut.loadApplications(for: .cambridge)

        XCTAssertEqual(sut.applications, expected)
        XCTAssertFalse(sut.isLoading)
        XCTAssertNil(sut.error)
    }

    func test_loadApplications_setsErrorOnFailure() async {
        spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

        await sut.loadApplications(for: .cambridge)

        XCTAssertTrue(sut.applications.isEmpty)
        XCTAssertEqual(sut.error, .networkUnavailable)
    }

    func test_selectApplication_notifiesCoordinator() async {
        var selectedId: PlanningApplicationId?
        sut.onApplicationSelected = { selectedId = $0 }

        sut.selectApplication(PlanningApplicationId("APP-001"))

        XCTAssertEqual(selectedId, PlanningApplicationId("APP-001"))
    }
}
```

### 4. Concurrency & State

Swift Concurrency provides structured, compiler-checked async code. Using `DispatchQueue` or `Combine` for simple async work bypasses these checks and makes data races harder to catch.

- **Pattern:** Swift Concurrency (`async`/`await`) exclusively.
- **No `DispatchQueue.main.async`** unless wrapping a legacy API that has no async alternative.
- **No completion handlers** for async logic — use `async throws` instead.
- **No `Combine`** for one-off async tasks. `Combine` is appropriate for reactive streams (e.g., observing a `@Published` property), not for request/response patterns.
- **`@MainActor`:** All UI-bound state in ViewModels must be annotated with `@MainActor`. This is enforced at compile time and eliminates a class of threading bugs.

### 5. Data Access

The app layer should speak in domain entities, never in persistence-specific types. This means the ViewModel asks for a `PlanningApplication` (domain struct), and the repository implementation handles the mapping from `SwiftData` models or API JSON.

- **Persistence:** SwiftData for local caching.
- **Abstraction:** Repository protocols are defined in the Domain package. Implementations live in the Data package and handle all SwiftData/API concerns internally.
- **Mapping:** Data-layer models (SwiftData `@Model` classes, API DTOs) are separate types. The repository maps between them and domain structs.

**Example — SwiftData Model (Data layer):**
```swift
@Model
final class PlanningApplicationRecord {
    @Attribute(.unique) var id: String
    var reference: String
    var authorityCode: String
    var status: String
    var receivedDate: Date
    var applicationDescription: String
    var address: String

    func toDomain() -> PlanningApplication {
        PlanningApplication(
            id: PlanningApplicationId(id),
            reference: ApplicationReference(reference),
            authority: LocalAuthority(code: authorityCode),
            status: ApplicationStatus(rawValue: status) ?? .unknown,
            receivedDate: receivedDate,
            description: applicationDescription,
            address: address
        )
    }
}
```

**Example — Repository Implementation (Data layer adapter):**
```swift
final class SwiftDataPlanningApplicationRepository: PlanningApplicationRepository {
    private let modelContext: ModelContext
    private let apiClient: PlanningAPIClient

    init(modelContext: ModelContext, apiClient: PlanningAPIClient) {
        self.modelContext = modelContext
        self.apiClient = apiClient
    }

    func fetchApplications(for authority: LocalAuthority) async throws -> [PlanningApplication] {
        let dto = try await apiClient.getApplications(authorityCode: authority.code)
        let records = dto.map { PlanningApplicationRecord(from: $0) }
        records.forEach { modelContext.insert($0) }
        try modelContext.save()
        return records.map { $0.toDomain() }
    }

    func fetchApplication(by id: PlanningApplicationId) async throws -> PlanningApplication {
        let predicate = #Predicate<PlanningApplicationRecord> { $0.id == id.value }
        let descriptor = FetchDescriptor(predicate: predicate)
        guard let record = try modelContext.fetch(descriptor).first else {
            throw DomainError.applicationNotFound(id)
        }
        return record.toDomain()
    }
}
```

## Workflow

### 1. Verification
To check the codebase for style and standards:

```bash
swiftlint lint --strict
swift test
```

### 2. Auto-Formatting
To automatically fix formatting issues:

```bash
swift-format format --in-place --recursive .
```

### 3. Setup Enforcements
To enforce standards in a project, use the bundled assets.

#### Apply .swiftlint.yml
Copy the standard `.swiftlint.yml` to the project root.

```bash
cp .claude/skills/ios-coding-standards/assets/.swiftlint.yml ./mobile/ios/
```

## Guidelines

### Naming Conventions
- **Types:** PascalCase (structs, enums, classes, protocols).
- **Properties/Functions:** camelCase.
- **Protocols:**
    - Capabilities: `...able` (e.g., `Codable`, `Searchable`).
    - Services/Repositories: `...Service`, `...Repository` (e.g., `PlanningApplicationRepository`).
    - No `I` prefix (that is a C# convention). Use a `Protocol` suffix only if there is a genuine name collision with a concrete type.

### Best Practices
- **No Force Unwraps:** `!` is forbidden outside of `XCTest` assertions. Force unwraps crash the app at runtime and bypass Swift's safety guarantees.
- **Final Classes:** Classes should be `final` by default. Only remove `final` when inheritance is explicitly designed and documented.
- **Access Control:** `private` by default. Expose only what is strictly necessary — a smaller public surface means fewer accidental breaking changes and easier refactoring.
- **View Logic:** Views render State from the ViewModel and forward user intents. They should contain zero business logic (no `if/else` on domain rules). Conditional rendering based on ViewModel state (e.g., showing a loading spinner) is fine.
