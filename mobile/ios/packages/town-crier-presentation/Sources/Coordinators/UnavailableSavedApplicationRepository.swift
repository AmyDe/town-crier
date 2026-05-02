import Foundation
import TownCrierDomain

/// Stand-in repository used when the Saved tab is constructed without a real
/// `SavedApplicationRepository`. Returns an empty list rather than crashing so
/// the empty state renders cleanly. The composition root in `TownCrierApp`
/// always injects a real repository — this is purely defensive for tests and
/// preview environments.
struct UnavailableSavedApplicationRepository: SavedApplicationRepository {
  func save(application: PlanningApplication) async throws {}
  func remove(applicationUid: String) async throws {}
  func loadAll() async throws -> [SavedApplication] { [] }
}
