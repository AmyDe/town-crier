/// Manages the user's saved applications. Not tier-gated.
public protocol SavedApplicationRepository: Sendable {
  func save(application: PlanningApplication) async throws
  func remove(applicationUid: String) async throws
  func loadAll() async throws -> [SavedApplication]
}
