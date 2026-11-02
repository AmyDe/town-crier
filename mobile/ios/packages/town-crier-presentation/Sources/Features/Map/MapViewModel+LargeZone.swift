import Foundation
import TownCrierDomain

extension MapViewModel {
  var showsLargeZoneLoadingMessage: Bool {
    isLoading && loadHasBeenSlow && selectedZone?.isLarge == true
  }
}
