/// The authentication method used to sign in.
public enum AuthMethod: String, Equatable, Sendable {
    case emailPassword
    case google
    case apple
    case unknown

    public var displayName: String {
        switch self {
        case .emailPassword: "Email & Password"
        case .google: "Google"
        case .apple: "Apple"
        case .unknown: "Unknown"
        }
    }
}
