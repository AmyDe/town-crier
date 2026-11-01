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
      return .development
    #else
      // TestFlight builds ship a *sandbox* App Store receipt; routing them to dev
      // exercises the dev backend on-device without affecting App Store builds.
      return environment(
        forReceiptLastPathComponent: Bundle.main.appStoreReceiptURL?.lastPathComponent)
    #endif
  }

  /// Maps the App Store receipt file name to an environment.
  /// - `"sandboxReceipt"` (TestFlight) → `.development`
  /// - `"receipt"` (App Store) → `.production`
  /// - `nil` (ad-hoc / local Release with no receipt) → `.production` (safe default)
  public static func environment(forReceiptLastPathComponent component: String?) -> APIEnvironment {
    component == "sandboxReceipt" ? .development : .production
  }
}
