import Foundation

/// Port for managing the user's server-side profile.
///
/// Maps to the `/v1/me` API endpoints:
/// - `create()` -> `POST /v1/me`
/// - `fetch()` -> `GET /v1/me`
/// - `update(...)` -> `PATCH /v1/me`
/// - `delete()` -> `DELETE /v1/me`
/// - `exportData()` -> `GET /v1/me/data`
public protocol UserProfileRepository: Sendable {
  /// Creates the user profile on the server. The API reads identity from the JWT.
  func create() async throws -> ServerProfile

  /// Fetches the current user profile. Returns `nil` if no profile exists (404).
  func fetch() async throws -> ServerProfile?

  /// Updates mutable profile preferences.
  ///
  /// `savedDecisionPush` and `savedDecisionEmail` default to `true` so existing
  /// callers that only manage digest preferences keep their behaviour without
  /// accidentally toggling saved-application notifications off.
  func update(
    pushEnabled: Bool,
    digestDay: DayOfWeek,
    emailDigestEnabled: Bool,
    savedDecisionPush: Bool,
    savedDecisionEmail: Bool
  ) async throws -> ServerProfile

  /// Deletes the user profile and cascades (removes all user data server-side).
  func delete() async throws

  /// Fetches the full GDPR data export as raw JSON bytes.
  ///
  /// The export is opaque to the client: the server bytes are returned as-is so
  /// the user can save or share a machine-readable copy of all their data. The
  /// caller must not decode or re-encode the payload, to keep the export
  /// byte-stable with what the server produced.
  func exportData() async throws -> Data
}
