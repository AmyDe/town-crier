/// A server-computed aggregate of planning applications for one grid cell of the
/// watch-zone map (GH#698). The API grids the in-viewport applications by zoom
/// level in PostGIS and returns one of these per non-empty cell, so the device
/// renders tens of aggregates instead of holding the whole zone's 22k pins.
///
/// When ``count`` is 1 the cell holds a single application, so ``member`` carries
/// its identity and ``statusCounts`` its lone status — enough for the client to
/// draw a status-coloured pin and, on tap, point-read the full record for the
/// summary sheet. When ``count`` exceeds 1 the cell is an amber count bubble; a
/// tap zooms in to spread its members into finer cells.
///
/// One multi-member case cannot be split by zoom: when the members are
/// coincident (or closer than the finest grid cell) the server marks the cell
/// "unsplittable" and lists those members in ``members`` (GH#722). Such a
/// ``isStacked`` cell can never resolve to individual pins, so a tap opens a
/// disambiguation list instead of zooming forever. ``members`` is empty for
/// every splittable cell, so existing single/bubble behaviour is unchanged.
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
  /// The carried member identities for an unsplittable (coincident) multi-member
  /// cell — populated only when zoom can no longer separate the members (GH#722).
  /// Empty for splittable cells, where a tap still zooms in. Drives ``isStacked``
  /// and the disambiguation list the client opens on tap.
  public let members: [PlanningApplicationId]

  public init(
    coordinate: Coordinate,
    count: Int,
    statusCounts: [ApplicationStatus: Int],
    member: PlanningApplicationId?,
    members: [PlanningApplicationId] = []
  ) {
    self.coordinate = coordinate
    self.count = count
    self.statusCounts = statusCounts
    self.member = member
    self.members = members
  }

  /// Whether this cell holds exactly one application (a real pin, not a bubble).
  public var isSingleMember: Bool {
    count == 1
  }

  /// Whether this is an unsplittable multi-member cell: more than one application
  /// stacked at (effectively) one location, with their identities carried in
  /// ``members``. A tap on such a cell opens the disambiguation list rather than
  /// zooming in — zoom could never separate coincident points (GH#722).
  public var isStacked: Bool {
    count > 1 && !members.isEmpty
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
