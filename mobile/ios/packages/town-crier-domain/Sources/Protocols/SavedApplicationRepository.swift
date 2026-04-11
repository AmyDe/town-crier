/// Port for managing the user's saved (bookmarked) planning applications.
///
/// Available to all tiers -- no entitlement gating.
public protocol SavedApplicationRepository: Sendable {
  func save(applicationUid: String) async throws
  func remove(applicationUid: String) async throws
  func loadAll() async throws -> [SavedApplication]
}
