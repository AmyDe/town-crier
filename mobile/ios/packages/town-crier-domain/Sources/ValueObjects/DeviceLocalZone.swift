import Foundation

/// A circular geographic area saved on-device before sign-up (GH#879 Phase
/// 4): name, centre, radius. Deliberately NOT a ``WatchZone`` — that type
/// requires a server-resolved `authorityId`, which an anonymous session has
/// no way to obtain. No alerts, no server persistence: a device-local zone
/// exists purely to drive the anonymous Applications list and Map while the
/// user has no account.
public struct DeviceLocalZone: Equatable, Hashable, Identifiable, Sendable {
  public let id: DeviceLocalZoneId
  public let name: String
  public let centre: Coordinate
  public let radiusMetres: Double

  /// Matches the public `GET /v1/applications/near-point` endpoint's own
  /// clamp — deliberately NOT the authed ``WatchZoneLimits`` tiers, since a
  /// device-local zone has no subscription tier to key off.
  public static let minRadiusMetres: Double = 100
  public static let maxRadiusMetres: Double = 5000

  /// The on-device cap. Matches the Personal tier's watch-zone quota so a
  /// post-signup conversion is never an absurd "delete some of your areas
  /// first" moment (GH#879 pre-resolved decision).
  public static let maxZoneCount = 3

  public init(
    id: DeviceLocalZoneId = DeviceLocalZoneId(),
    name: String,
    centre: Coordinate,
    radiusMetres: Double
  ) throws {
    let trimmed = name.trimmingCharacters(in: .whitespaces)
    guard !trimmed.isEmpty else {
      throw DomainError.invalidWatchZoneName
    }
    self.id = id
    self.name = trimmed
    self.centre = centre
    self.radiusMetres = Self.clampRadius(radiusMetres)
  }

  /// Clamps `metres` to `[minRadiusMetres, maxRadiusMetres]`.
  public static func clampRadius(_ metres: Double) -> Double {
    min(max(metres, minRadiusMetres), maxRadiusMetres)
  }
}
