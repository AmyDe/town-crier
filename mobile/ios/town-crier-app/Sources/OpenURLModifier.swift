import SwiftUI
import TownCrierData
import TownCrierPresentation

extension View {
  /// Primary inbound entry point for both Auth0 callbacks and Universal
  /// Links (tc-28x2, GH #763 Problem 1, second attempt). In a
  /// SwiftUI-lifecycle app, inbound Universal Links are delivered to
  /// `.onOpenURL`, NOT `.onContinueUserActivity` — the first attempt
  /// (`UniversalLinkModifier`, kept as belt-and-braces) never fired
  /// on-device because iOS never routed through that path here. `.onOpenURL`
  /// also receives the Auth0 login/logout callback, so the URL is tried
  /// against `OpenURLRoute.resolve` (== the already-tested
  /// `UniversalLinkParser.parse`) first; only a non-match falls through to
  /// `AuthCallbackHandler`.
  func handlingOpenURL(coordinator: AppCoordinator) -> some View {
    modifier(OpenURLModifier(coordinator: coordinator))
  }
}

private struct OpenURLModifier: ViewModifier {
  @ObservedObject var coordinator: AppCoordinator

  func body(content: Content) -> some View {
    content
      .onOpenURL { url in
        switch OpenURLRoute.resolve(url) {
        case .universalLink(let deepLink):
          UniversalLinkDeliveryLogger.logDelivery(
            source: "SwiftUI onOpenURL", url: url, deepLink: deepLink)
          coordinator.handleDeepLink(deepLink)
        case .other(let url):
          AuthCallbackHandler.handle(url: url)
        }
      }
  }
}
