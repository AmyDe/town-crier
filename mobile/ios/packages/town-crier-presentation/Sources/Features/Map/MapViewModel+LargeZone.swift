import Foundation
import TownCrierDomain

extension MapViewModel {
  var showsLargeZoneNotice: Bool {
    selectedZone?.isLarge == true
  }

  var showsLargeZoneLoadingMessage: Bool {
    isLoading && loadHasBeenSlow && showsLargeZoneNotice
  }
}
