/// The result of fetching the user's application authorities.
///
/// Each authority is derived from the user's watch zones on the server.
public struct ApplicationAuthorityResult: Equatable, Sendable {
  public let authorities: [LocalAuthority]
  public let count: Int

  public init(authorities: [LocalAuthority], count: Int) {
    self.authorities = authorities
    self.count = count
  }
}
