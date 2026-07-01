import Foundation
import TownCrierDomain

extension AppCoordinator {
  /// Routes a parsed ``DeepLink`` to the appropriate presentation. Used by
  /// both the push-notification delegate and the Universal Links handler.
  public func handleDeepLink(_ deepLink: DeepLink) {
    deepLinkError = nil
    switch deepLink {
    case .applicationDetail(let id):
      // Switch to the Applications tab so the hoisted detail sheet
      // modifier (TownCrierApp.swift) is in scope when SwiftUI evaluates
      // the resulting `detailApplication` mutation. Without this, taps
      // arriving while the user was on Saved/Map/Zones would foreground
      // the app on the previous tab and the sheet would never present
      // (tc-dt3x).
      selectedTab = .applications
      // Opening an application from a push/deep link is the instant-alert payoff
      // moment — a fire-eligible review signal (GH #628). This is deliberately
      // distinct from browsing the in-app list.
      reviewPromptTracker?.record(.openedAlert)
      showApplicationDetail(id)
    case .applicationsList:
      selectedTab = .applications
    case let .shareApplication(authoritySlug, ref):
      // Inbound public share Universal Link (GH #738 Slice 4). Switch to the
      // Applications tab so the hoisted detail sheet modifier is in scope when
      // SwiftUI evaluates the `detailApplication` mutation, then resolve the
      // application via the anonymous by-slug read. No review-prompt signal —
      // arriving from a shared web link is not the instant-alert payoff moment.
      selectedTab = .applications
      showApplicationDetail(bySlug: authoritySlug, ref: ref)
    }
  }
}
