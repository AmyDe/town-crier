import Foundation

/// A circular geographic area that a user monitors for planning applications.
public struct WatchZone: Equatable, Hashable, Identifiable, Sendable {
  public let id: WatchZoneId
  public let name: String
  public let centre: Coordinate
  public let radiusMetres: Double
  public let pushEnabled: Bool
  public let emailInstantEnabled: Bool
  /// Whether this zone currently exceeds the user's effective tier quota and
  /// has stopped generating new notifications (GH#889 P1/P2). Purely a
  /// server-derived display flag — never computed or mutated by the client.
  /// A paused zone remains fully listed, editable, and deletable; it is
  /// automatically revived (server-side, with no client action) once the
  /// user upgrades or deletes older zones.
  public let paused: Bool
  /// The custom-shape polygon for this zone, or `nil` for a circle
  /// (GH#1031). `centre`/`radiusMetres` remain always-present: for a
  /// custom-shape zone they hold the polygon's derived centroid and
  /// enclosing radius, so every existing circle-shaped read path (map
  /// centring, list rows, distance sort) keeps working unchanged. A
  /// non-nil boundary is the sole "this is a custom shape" discriminator —
  /// see ``isCustomShape``.
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
  /// coordinate. Used by the Applications screen's distance sort
  /// (tc-mso6) and any future "near me"-style features that need a
  /// stable comparator across the domain layer.
  public func distance(to coordinate: Coordinate) -> Double {
    centre.distanceMetres(to: coordinate)
  }

  /// Whether this is a custom-shape (polygon) zone rather than a circle
  /// (GH#1031). Mirrors the server's `WatchZone.IsCustomShape()`.
  public var isCustomShape: Bool {
    boundary != nil
  }
}
