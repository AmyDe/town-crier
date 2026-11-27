import Foundation

/// A geographic area, a circle or a custom polygon, that a user monitors for planning applications.
public struct WatchZone: Equatable, Hashable, Identifiable, Sendable {
  public let id: WatchZoneId
  public let name: String
  public let centre: Coordinate
  public let radiusMetres: Double
  public let pushEnabled: Bool
  public let emailInstantEnabled: Bool
  /// Server-derived display flag: the zone exceeds the user's tier quota and
  /// generates no new notifications. The client never computes it.
  public let paused: Bool
  /// `nil` for a circle. For a custom shape, `centre`/`radiusMetres` hold the
  /// polygon's centroid and enclosing radius.
  public let boundary: WatchZoneBoundary?

  public init(
    id: WatchZoneId = WatchZoneId(),
    name: String,
    centre: Coordinate,
    radiusMetres: Double,
    pushEnabled: Bool = true,
    emailInstantEnabled: Bool = true,
    paused: Bool = false,
    boundary: WatchZoneBoundary? = nil
  ) throws {
    let trimmed = name.trimmingCharacters(in: .whitespaces)
    guard !trimmed.isEmpty else {
      throw DomainError.invalidWatchZoneName
    }
    guard radiusMetres > 0 else {
      throw DomainError.invalidWatchZoneRadius
    }
    self.id = id
    self.name = trimmed
    self.centre = centre
    self.radiusMetres = radiusMetres
    self.pushEnabled = pushEnabled
    self.emailInstantEnabled = emailInstantEnabled
    self.paused = paused
    self.boundary = boundary
  }

  /// Convenience initializer that derives the zone name from a validated postcode.
  public init(
    id: WatchZoneId = WatchZoneId(),
    postcode: Postcode,
    centre: Coordinate,
    radiusMetres: Double,
    pushEnabled: Bool = true,
    emailInstantEnabled: Bool = true,
    paused: Bool = false,
    boundary: WatchZoneBoundary? = nil
  ) throws {
    try self.init(
      id: id,
      name: postcode.value,
      centre: centre,
      radiusMetres: radiusMetres,
      pushEnabled: pushEnabled,
      emailInstantEnabled: emailInstantEnabled,
      paused: paused,
      boundary: boundary
    )
  }

  /// Returns true if the given coordinate falls within this watch zone.
  public func contains(_ coordinate: Coordinate) -> Bool {
    centre.distanceMetres(to: coordinate) <= radiusMetres
  }

  /// Great-circle distance in metres from this zone's centre to the given
  /// coordinate.
  public func distance(to coordinate: Coordinate) -> Double {
    centre.distanceMetres(to: coordinate)
  }

  /// Whether this is a custom-shape (polygon) zone rather than a circle.
  /// Mirrors the server's `WatchZone.IsCustomShape()`.
  public var isCustomShape: Bool {
    boundary != nil
  }

  /// Whether this zone is large under ``LargeZone``'s area rule. A custom
  /// shape uses its polygon area, not the derived enclosing radius.
  public var isLarge: Bool {
    if let boundary {
      return LargeZone.isLarge(boundary: boundary)
    }
    return LargeZone.isLarge(radiusMetres: radiusMetres)
  }
}
