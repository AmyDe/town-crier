# Data Access & SwiftData (reference)

Read when the bead touches persistence, SwiftData, repository implementations, or DTO↔domain mapping. The core (`SKILL.md`) states the abstraction rule; this file is the full detail and examples.

## Data Access

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
