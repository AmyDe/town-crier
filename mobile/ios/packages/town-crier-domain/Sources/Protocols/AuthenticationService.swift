/// Port for authentication operations.
/// Implementations handle login, logout, token refresh, and session retrieval
/// via an external identity provider.
public protocol AuthenticationService: Sendable {
  /// Presents the login/registration UI and returns the resulting session.
  func login() async throws -> AuthSession

  /// Clears the current session and revokes tokens.
  func logout() async throws

  /// Refreshes an expired session, returning a new valid session.
  func refreshSession() async throws -> AuthSession

  /// Returns the current stored session, or nil if the user is not authenticated.
  func currentSession() async -> AuthSession?

  /// Permanently deletes the user's account and clears the local session.
  func deleteAccount() async throws
}
