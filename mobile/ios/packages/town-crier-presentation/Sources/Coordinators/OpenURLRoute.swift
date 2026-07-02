import Foundation

/// The two possible fates of a URL handed to SwiftUI's `.onOpenURL`
/// (tc-28x2, GH #763 Problem 1, second attempt).
///
/// Root cause of the original bug: in a SwiftUI-lifecycle app, inbound
/// Universal Links are delivered to `.onOpenURL`, not
/// `.onContinueUserActivity` -- but `.onOpenURL` (TownCrierApp.swift)
/// unconditionally handed every URL to `AuthCallbackHandler.handle`, which
/// resumes Auth0's `WebAuthentication` and silently returns `false` for a
/// non-auth URL. A tapped share link therefore launched the app but never
/// reached `UniversalLinkParser`/`AppCoordinator.handleDeepLink`.
///
/// `resolve(_:)` is the pure decision `.onOpenURL` delegates to: try the
/// existing, already-tested `UniversalLinkParser.parse(_:)` first; only if
/// it returns `nil` (as it does for the Auth0 callback URL and everything
/// else) does the URL fall through to the auth handler. Extracted into this
/// package (rather than left inline in the app target's closure) because
/// `town-crier-app/Sources` is not covered by `swift test` at all, and this
/// is the only way to drive the routing decision under TDD.
public enum OpenURLRoute: Equatable {
  case universalLink(DeepLink)
  case other(URL)

  public static func resolve(_ url: URL) -> OpenURLRoute {
    if let deepLink = UniversalLinkParser.parse(url) {
      return .universalLink(deepLink)
    }
    return .other(url)
  }
}
