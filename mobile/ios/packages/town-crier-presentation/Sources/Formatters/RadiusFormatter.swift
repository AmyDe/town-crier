import Foundation

/// Formats a radius in metres for user-facing display.
///
/// - Values under 1000 m show as whole metres (e.g. "500 m").
/// - Values of 1000 m or above show as kilometres, with one decimal
///   place for fractional values (e.g. "1.5 km") and no decimal
///   for whole values (e.g. "2 km").
public func formatRadius(_ metres: Double) -> String {
  if metres >= 1000 {
    let km = metres / 1000
    if km.truncatingRemainder(dividingBy: 1) == 0 {
      return "\(Int(km)) km"
    }
    return String(format: "%.1f km", km)
  }
  return "\(Int(metres)) m"
}
