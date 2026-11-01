import Foundation

public enum LargeZone {
  public static let areaThresholdSquareMetres: Double = 0

  public static func isLarge(radiusMetres: Double) -> Bool { false }

  public static func isLarge(boundary: WatchZoneBoundary) -> Bool { false }

  public static func areaSquareMetres(of boundary: WatchZoneBoundary) -> Double { 0 }
}
