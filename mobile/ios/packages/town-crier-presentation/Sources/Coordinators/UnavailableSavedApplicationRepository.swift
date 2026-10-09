import Foundation
import TownCrierDomain

/// Defensive stand-in returning an empty list when the Saved tab is built
/// without a real `SavedApplicationRepository` (tests and previews).
struct UnavailableSavedApplicationRepository: SavedApplicationRepository {
  func save(application: PlanningApplication) async throws {}
  func remove(applicationUid: String) async throws {}
  func loadAll() async throws -> [SavedApplication] { [] }
}
