import Foundation
import TownCrierDomain

extension AppCoordinator {
  /// Routes a parsed ``DeepLink`` to the appropriate presentation. Used by
  /// both the push-notification delegate and the Universal Links handler.
  public func handleDeepLink(_ deepLink: DeepLink) {
    deepLinkError = nil
    switch deepLink {
    case .applicationDetail(let id):
      showApplicationDetail(id)
    case .applicationsList:
      selectedTab = .applications
    }
  }
}
