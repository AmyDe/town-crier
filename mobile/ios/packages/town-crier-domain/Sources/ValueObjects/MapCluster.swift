/// A server-computed aggregate of planning applications for one grid cell of the
/// watch-zone map (GH#698). The API grids the in-viewport applications by zoom
/// level in PostGIS and returns one of these per non-empty cell, so the device
/// renders tens of aggregates instead of holding the whole zone's 22k pins.
///
/// When ``count`` is 1 the cell holds a single application, so ``member`` carries
/// its identity and ``statusCounts`` its lone status — enough for the client to
/// draw a status-coloured pin and, on tap, point-read the full record for the
/// summary sheet. When ``count`` exceeds 1 the cell is an amber count bubble and
/// ``member`` is nil; a tap zooms in.
public struct MapCluster: Identifiable, Equatable, Sendable {
  /// The cell centroid — the mean position of the cell's member applications.
  public let coordinate: Coordinate
  /// The number of applications collapsed into this cell.
  public let count: Int
  /// Per-status breakdown of the cell's members; the values sum to ``count``.
  public let statusCounts: [ApplicationStatus: Int]
  /// The single member's identity, present iff ``count`` is 1. Lets a single-pin
  /// tap route to the summary sheet via a one-row point read with no held set.
  public let member: PlanningApplicationId?

  public init(
    coordinate: Coordinate,
    count: Int,
    statusCounts: [ApplicationStatus: Int],
    member: PlanningApplicationId?
  ) {
    self.coordinate = coordinate
    self.count = count
    self.statusCounts = statusCounts
    self.member = member
  }

  /// Whether this cell holds exactly one application (a real pin, not a bubble).
  public var isSingleMember: Bool {
    count == 1
  }

  /// The lone application's status for a single-member cell — drives the
  /// status-coloured pin. Nil for multi-member cells, which render as a bubble.
  public var memberStatus: ApplicationStatus? {
    guard isSingleMember else { return nil }
    return statusCounts.first?.key
  }

  /// A stable identity for annotation diffing: the member id for single cells,
  /// else the centroid plus count so distinct bubbles never collide.
  public var id: String {
    if let member {
      return member.value
    }
    return "cluster:\(coordinate.latitude):\(coordinate.longitude):\(count)"
  }
}
