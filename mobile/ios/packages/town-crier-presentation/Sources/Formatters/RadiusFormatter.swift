import Foundation

/// Formats a radius in metres for user-facing display.
///
/// - Values under 1000 m show as whole metres (e.g. "500m").
/// - Values of 1000 m or above show as kilometres, with one decimal
///   place for fractional values (e.g. "1.5km") and no decimal
///   for whole values (e.g. "2km").
///
/// No space before the unit: this string renders in `TCTypography.mono`
/// (SF Mono), where the space glyph is digit-width and reads as a visible
/// double space (GH#912 Phase 1).
public func formatRadius(_ metres: Double) -> String {
  if metres >= 1000 {
    let km = metres / 1000
    if km.truncatingRemainder(dividingBy: 1) == 0 {
      return "\(Int(km))km"
    }
    return String(format: "%.1fkm", km)
  }
  return "\(Int(metres))m"
}
