import Foundation

public enum APIEnvironment: Equatable, Sendable {
  case development
  case production

  // swiftlint:disable:next force_unwrapping
  private static let developmentURL = URL(string: "https://api-dev.towncrierapp.uk")!
  // swiftlint:disable:next force_unwrapping
  private static let productionURL = URL(string: "https://api.towncrierapp.uk")!

  public var baseURL: URL {
    switch self {
    case .development:
      Self.developmentURL
    case .production:
      Self.productionURL
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
