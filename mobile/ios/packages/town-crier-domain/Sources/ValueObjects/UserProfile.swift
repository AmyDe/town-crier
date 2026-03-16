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
}
