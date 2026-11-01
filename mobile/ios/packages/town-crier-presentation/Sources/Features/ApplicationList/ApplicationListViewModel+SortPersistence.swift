import Foundation
import TownCrierDomain

extension ApplicationListViewModel {
  static func readPersistedSort(
    from defaults: UserDefaults,
    key: String
  ) -> ApplicationsSort {
    if let raw = defaults.string(forKey: key),
      let parsed = ApplicationsSort(rawValue: raw) {
      return parsed
    }
    return .recentActivity
  }
}
