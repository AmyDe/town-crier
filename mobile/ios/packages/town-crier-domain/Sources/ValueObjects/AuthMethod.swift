/// The authentication method used to sign in.
public enum AuthMethod: String, Equatable, Sendable {
    case emailPassword
    case google
    case apple
    case unknown
}
