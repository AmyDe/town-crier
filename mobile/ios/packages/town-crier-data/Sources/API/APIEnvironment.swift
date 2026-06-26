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
      // TestFlight builds ship a *sandbox* App Store receipt; App Store builds ship a
      // production receipt. Routing TestFlight to dev lets us exercise the dev backend
      // on-device without affecting the public App Store build. `Bundle.main` is thin,
      // untested glue; the decision lives in the pure seam below.
      return environment(
        forReceiptLastPathComponent: Bundle.main.appStoreReceiptURL?.lastPathComponent)
    #endif
  }

  /// Pure decision seam for the release-build branch of ``current``, extracted so the
  /// sandbox-vs-production choice is unit-testable without touching `Bundle.main`.
  /// - `"sandboxReceipt"` (TestFlight) → `.development`
  /// - `"receipt"` (App Store) → `.production`
  /// - `nil` (ad-hoc / local Release with no receipt) → `.production` (safe default)
  public static func environment(forReceiptLastPathComponent component: String?) -> APIEnvironment {
    component == "sandboxReceipt" ? .development : .production
  }
}
