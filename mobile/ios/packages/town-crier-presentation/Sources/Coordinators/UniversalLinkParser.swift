import Foundation
import TownCrierDomain

/// Parses Universal Link URLs (handed to the app by
/// `NSUserActivityTypeBrowsingWeb`) into the in-app ``DeepLink`` vocabulary.
///
/// The web app exposes two relevant routes that the AASA file claims:
/// `/applications/{uid}` (a specific planning application) and `/applications`
/// (the root list). PlanIt UIDs may contain `/` so the entire path suffix
/// after `/applications/` is preserved verbatim.
public enum UniversalLinkParser {
  private static let applicationsPath = "/applications"

  public static func parse(_ url: URL) -> DeepLink? {
    let path = url.path
    guard path.hasPrefix(applicationsPath) else { return nil }
    let suffix = String(path.dropFirst(applicationsPath.count))
    if suffix.isEmpty {
      return .applicationsList
    }
    guard suffix.hasPrefix("/") else { return nil }
    let uid = String(suffix.dropFirst())
    guard !uid.isEmpty else { return .applicationsList }
    return .applicationDetail(PlanningApplicationId(uid))
  }
}
