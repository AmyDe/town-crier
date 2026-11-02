import Foundation
import TownCrierDomain

extension ApplicationListViewModel {
  var showsLargeZoneLoadingMessage: Bool {
    isLoading && loadHasBeenSlow && (selectedZone ?? zone)?.isLarge == true
  }
}
