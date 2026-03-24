import Foundation

public enum APIEnvironment: Equatable, Sendable {
    case development
    case production

    public var baseURL: URL {
        switch self {
        case .development:
            URL(string: "https://api.dev.towncrierapp.uk")!
        case .production:
            URL(string: "https://api.towncrierapp.uk")!
        }
    }

    public static var current: APIEnvironment {
        #if DEBUG
        .development
        #else
        .production
        #endif
    }
}
