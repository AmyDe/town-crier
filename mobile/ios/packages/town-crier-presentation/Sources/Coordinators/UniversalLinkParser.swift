import Foundation
import TownCrierDomain

/// Parses Universal Link URLs (handed to the app by
/// `NSUserActivityTypeBrowsingWeb`) into the in-app ``DeepLink`` vocabulary.
///
/// Two schemes are recognised:
/// - the public share scheme `/a/{authoritySlug}/{ref...}` (GH #738 Slice 4),
///   where `ref` is the application's full area-prefixed PlanIt name, verbatim
///   (slashes preserved as path separators);
/// - the legacy `/applications/{uid}` (a specific planning application) and
///   `/applications` (the root list). PlanIt UIDs may contain `/`, so the
///   entire path suffix after `/applications/` is preserved verbatim.
public enum UniversalLinkParser {
  private static let applicationsPath = "/applications"
  private static let sharePrefix = "/a/"

  public static func parse(_ url: URL) -> DeepLink? {
    let path = url.path
    if let shareLink = parseShare(path) {
      return shareLink
    }
    return parseApplications(path)
  }

  /// Convenience overload for the raw `NSUserActivity` continuation object
  /// both SwiftUI's `.onContinueUserActivity` and the UIKit-level
  /// `application(_:continue:restorationHandler:)` fallback receive
  /// (tc-28x2). Guards the activity type and unwraps `webpageURL` before
  /// delegating to ``parse(_:URL)`` — the URL-path parsing logic is not
  /// duplicated.
  public static func parse(_ activity: NSUserActivity) -> DeepLink? {
    guard activity.activityType == NSUserActivityTypeBrowsingWeb,
      let url = activity.webpageURL
    else { return nil }
    return parse(url)
  }

  /// Parses `/a/{authoritySlug}/{ref...}`: the first segment after `/a/` is the
  /// slug, everything after it (verbatim, slashes preserved) is the ref. A bare
  /// `/a` or `/a/`, or a slug with no ref, returns `nil`. The `/a/` separator is
  /// required, so `/afoo` does not match.
  private static func parseShare(_ path: String) -> DeepLink? {
    guard path.hasPrefix(sharePrefix) else { return nil }
    let suffix = String(path.dropFirst(sharePrefix.count))
    let parts = suffix.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)
    guard parts.count == 2, !parts[0].isEmpty, !parts[1].isEmpty else { return nil }
    return .shareApplication(authoritySlug: String(parts[0]), ref: String(parts[1]))
  }

  private static func parseApplications(_ path: String) -> DeepLink? {
    guard path.hasPrefix(applicationsPath) else { return nil }
    let suffix = String(path.dropFirst(applicationsPath.count))
    if suffix.isEmpty {
      return .applicationsList
    }
    guard suffix.hasPrefix("/") else { return nil }
    let uid = String(suffix.dropFirst())
    guard !uid.isEmpty else { return .applicationsList }
    // Universal Links only carry the uid path segment — split on first "/" to
    // reconstruct authority + name. Legacy UIDs without a "/" are treated as
    // name-only with empty authority. This parser is best-effort; if the URL
    // does not carry authority, the fetch will fail gracefully.
    let components = uid.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)
    let authority = components.count > 1 ? String(components[0]) : ""
    let name = components.count > 1 ? String(components[1]) : uid
    return .applicationDetail(PlanningApplicationId(authority: authority, name: name))
  }
}
