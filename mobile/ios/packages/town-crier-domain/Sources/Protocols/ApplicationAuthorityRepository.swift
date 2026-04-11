/// Port for fetching the user's application authorities.
///
/// Maps to `GET /v1/me/application-authorities`. Returns the set of local
/// authorities derived from the user's watch zones.
public protocol ApplicationAuthorityRepository: Sendable {
  /// Fetches the authorities associated with the current user's watch zones.
  func fetchAuthorities() async throws -> ApplicationAuthorityResult
}
