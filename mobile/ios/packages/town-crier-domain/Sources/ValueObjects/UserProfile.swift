/// A user's identity information from the authentication provider.
public struct UserProfile: Equatable, Sendable {
  public let userId: String
  public let email: String
  public let name: String?

  public init(userId: String, email: String, name: String? = nil) {
    self.userId = userId
    self.email = email
    self.name = name
  }

  /// Derives the authentication method from the userId prefix.
  public var authMethod: AuthMethod {
    if userId.hasPrefix("auth0|") {
      return .emailPassword
    } else if userId.hasPrefix("google-oauth2|") {
      return .google
    } else if userId.hasPrefix("apple|") {
      return .apple
    } else {
      return .unknown
    }
  }
}
