/// Port for managing the user's saved (bookmarked) planning applications.
///
/// Available to all tiers -- no entitlement gating.
///
/// `save` carries the full ``PlanningApplication`` so the API can upsert the
/// canonical record into Cosmos at save time (see bead tc-if12).
public protocol SavedApplicationRepository: Sendable {
  func save(application: PlanningApplication) async throws
  func remove(applicationUid: String) async throws
  func loadAll() async throws -> [SavedApplication]
}
