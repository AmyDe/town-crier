import Foundation

/// DTO for parsing the `{error, message}` envelope the watch-zone endpoints
/// return on `400`/`409`.
///
/// The API returns e.g. `{"error": "boundary_too_large", "message": "..."}`
/// or `{"error": "zone_name_taken", "message": "..."}`. Internal to the data
/// layer -- callers see `DomainError.invalidWatchZoneBoundaryTooLarge` /
/// `DomainError.watchZoneNameTaken` (GH#1085).
struct WatchZoneErrorBody: Decodable, Sendable {
  let error: String
  let message: String?
}
