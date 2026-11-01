import Foundation
import TownCrierDomain

extension OnboardingViewModel {
  /// Whether to show the large-zone warning: the radius in circle mode, the
  /// drawn polygon in custom mode (false until the vertices form a valid
  /// boundary). See ``LargeZone``.
  public var showsLargeZoneWarning: Bool {
    switch shapeMode {
    case .circle:
      return LargeZone.isLarge(radiusMetres: selectedRadiusMetres)
    case .custom:
      guard let boundary = try? WatchZoneBoundary(vertices: boundaryVertices) else { return false }
      return LargeZone.isLarge(boundary: boundary)
    }
  }
}
