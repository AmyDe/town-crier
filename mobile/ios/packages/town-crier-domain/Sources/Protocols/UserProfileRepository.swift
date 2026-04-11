/// Port for managing the user's server-side profile.
///
/// Maps to the `/v1/me` API endpoints:
/// - `create()` -> `POST /v1/me`
/// - `fetch()` -> `GET /v1/me`
/// - `update(...)` -> `PATCH /v1/me`
/// - `delete()` -> `DELETE /v1/me`
public protocol UserProfileRepository: Sendable {
  /// Creates the user profile on the server. The API reads identity from the JWT.
  func create() async throws -> ServerProfile

  /// Fetches the current user profile. Returns `nil` if no profile exists (404).
  func fetch() async throws -> ServerProfile?

  /// Updates mutable profile preferences.
  func update(
    pushEnabled: Bool,
    digestDay: DayOfWeek,
    emailDigestEnabled: Bool
  ) async throws -> ServerProfile

  /// Deletes the user profile and cascades (removes all user data server-side).
  func delete() async throws
}
