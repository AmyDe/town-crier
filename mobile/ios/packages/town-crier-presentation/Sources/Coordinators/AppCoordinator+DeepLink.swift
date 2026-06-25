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
    }
  }
}
