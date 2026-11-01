import Foundation
import TownCrierDomain

extension ApplicationListViewModel {
  var showsLargeZoneNotice: Bool {
    (selectedZone ?? zone)?.isLarge == true
  }

  var showsLargeZoneLoadingMessage: Bool {
    isLoading && loadHasBeenSlow && showsLargeZoneNotice
  }
}
