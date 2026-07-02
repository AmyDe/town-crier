# Testing Strategy: TDD & XCTest (reference)

Read when writing any test, spy, fake, or fixture. The core (`SKILL.md`) states the test-double conventions; this file is the full rationale and examples.

## Testing Strategy (TDD & XCTest)

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
