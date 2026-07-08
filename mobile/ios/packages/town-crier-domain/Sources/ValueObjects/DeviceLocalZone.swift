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

  /// The on-device cap. Deliberately 1, matching the Free tier's single
  /// server zone (GH#888 — reverses GH#879 Phase 4's "match Personal tier"
  /// pre-resolved decision). That earlier 3-zone cap made sign-up a net LOSS:
  /// converting to a Free account could only keep one of three device-local
  /// areas, and the very first extra-zone save hit
  /// `DomainError.insufficientEntitlement`, bouncing a brand-new account
  /// straight into the paywall. A cap of 1 means anonymous browsing never
  /// promises more than a free account actually gets.
  public static let maxZoneCount = 1

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
