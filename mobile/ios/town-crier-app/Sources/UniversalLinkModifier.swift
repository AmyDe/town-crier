import SwiftUI
import TownCrierPresentation

extension View {
  /// Wires SwiftUI's inbound Universal Link entry point (tc-28x2, GH #763
  /// Problem 1): continues a browsing activity into the existing
  /// `UniversalLinkParser` / `AppCoordinator.handleDeepLink` seam, and
  /// ensures the activity is routed into this app's single window rather
  /// than risking no window being considered eligible to handle it — a
  /// documented cause of `.onContinueUserActivity` silently not firing.
  func handlingUniversalLinks(coordinator: AppCoordinator) -> some View {
    modifier(UniversalLinkModifier(coordinator: coordinator))
  }
}

private struct UniversalLinkModifier: ViewModifier {
  @ObservedObject var coordinator: AppCoordinator

  func body(content: Content) -> some View {
    content
      .onContinueUserActivity(NSUserActivityTypeBrowsingWeb) { activity in
        guard let url = activity.webpageURL, let deepLink = UniversalLinkParser.parse(url) else {
          return
        }
        coordinator.handleDeepLink(deepLink)
      }
      // `TownCrierApp` has one `WindowGroup` and no iPad multi-window
      // support, so "prefer and allow everything" is unconditionally safe.
      .handlesExternalEvents(preferring: Set(["*"]), allowing: Set(["*"]))
  }
}
