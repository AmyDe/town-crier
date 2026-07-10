/// A server-computed aggregate of planning applications for one grid cell of
/// the anonymous (pre-signup) map (GH#924 Phase 2) — the anonymous mirror of
/// ``MapCluster``, backed by the public `GET /v1/applications/clusters`
/// endpoint rather than the authed per-zone clusters endpoint.
///
/// Kept as a distinct type rather than extending ``MapCluster`` so the
/// anonymous surface never depends on the authenticated one: an
/// ``AnonymousClusterMember`` carries the ``AnonymousClusterMember/authoritySlug``
/// the anonymous by-slug point-read needs, which ``PlanningApplicationId``
/// deliberately does not carry — adding an optional slug there would poison
/// its `Equatable`/`Hashable` identity (same application, slug present vs
/// absent, comparing unequal and breaking annotation diffing). See the
/// GH#924 issue's Pre-Resolved Design Decisions.
///
/// Carries the same derived helpers as ``MapCluster`` (``isSingleMember``,
/// ``isStacked``, ``memberStatus``, stable ``id``) for identical rendering
/// and tap-routing behaviour on the anonymous map.
public struct AnonymousMapCluster: Identifiable, Equatable, Sendable {
  /// The cell centroid — the mean position of the cell's member applications.
  public let coordinate: Coordinate
  /// The number of applications collapsed into this cell.
  public let count: Int
  /// Per-status breakdown of the cell's members; the values sum to ``count``.
  public let statusCounts: [ApplicationStatus: Int]
  /// The single member's identity, present iff ``count`` is 1. Lets a
  /// single-pin tap route to the summary sheet via a by-slug point read.
  public let member: AnonymousClusterMember?
  /// The carried member identities for an unsplittable (coincident)
  /// multi-member cell — populated only when zoom can no longer separate the
  /// members. Empty for splittable cells, where a tap still zooms in. Drives
  /// ``isStacked`` and the disambiguation list the client opens on tap.
  public let members: [AnonymousClusterMember]

  public init(
    coordinate: Coordinate,
    count: Int,
    statusCounts: [ApplicationStatus: Int],
    member: AnonymousClusterMember?,
    members: [AnonymousClusterMember] = []
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

  /// Whether this is an unsplittable multi-member cell: more than one
  /// application stacked at (effectively) one location, with their
  /// identities carried in ``members``. A tap on such a cell opens the
  /// disambiguation list rather than zooming in — zoom could never separate
  /// coincident points.
  public var isStacked: Bool {
    count > 1 && !members.isEmpty
  }

  /// The lone application's status for a single-member cell — drives the
  /// status-coloured pin. Nil for multi-member cells, which render as a bubble.
  public var memberStatus: ApplicationStatus? {
    guard isSingleMember else { return nil }
    return statusCounts.first?.key
  }

  /// A stable identity for annotation diffing: the member's identity for
  /// single cells, else the centroid plus count so distinct bubbles never
  /// collide.
  public var id: String {
    if let member {
      return member.id
    }
    return "cluster:\(coordinate.latitude):\(coordinate.longitude):\(count)"
  }
}

/// A cluster member's identity on the anonymous map — mirrors
/// ``PlanningApplicationId`` plus the ``authoritySlug`` the anonymous by-slug
/// point-read (`AnonymousApplicationDetailRepository.fetchApplication(bySlug:ref:)`)
/// needs. Cluster members carry the authority *area id* on the wire, not the
/// slug; the anonymous clusters handler resolves and attaches the slug
/// server-side before this decodes.
public struct AnonymousClusterMember: Equatable, Hashable, Sendable {
  /// The local authority area ID, expressed as a decimal string (e.g. `"314"`).
  public let authority: String
  /// The PlanIt case reference (e.g. `"Kingston/22/1234/FUL"`). This is the
  /// `ref` a stacked-cell or single-pin tap passes to
  /// `AnonymousApplicationDetailRepository.fetchApplication(bySlug:ref:)` —
  /// the by-slug endpoint resolves the authority from the slug itself, so
  /// `ref` must be bare, with NO authority-id prefix (verified live against
  /// dev: `by-slug/kingston/Kingston/26/01332/CPU` → 200,
  /// `by-slug/kingston/314/Kingston/26/01332/CPU` → 404). Same contract the
  /// authed by-slug call site uses — see
  /// `APIPlanningApplicationRepository.fetchApplication(bySlug:ref:)`'s "ref
  /// interpolates raw exactly like id.name" comment.
  public let name: String
  /// The URL-safe authority slug for the by-slug point read. Empty when the
  /// server could not resolve one (should never happen in practice — the
  /// static authorities table covers every real authority).
  public let authoritySlug: String

  public init(authority: String, name: String, authoritySlug: String) {
    self.authority = authority
    self.name = name
    self.authoritySlug = authoritySlug
  }

  /// The canonical slash-joined string form of the identifier
  /// (e.g. `"314/Kingston/22/1234/FUL"`) — the same identity contract
  /// ``PlanningApplicationId/value`` uses. NOT the by-slug `ref` (that's the
  /// bare ``name`` — see its docs); this is for display/logging/diffing only.
  public var value: String {
    "\(authority)/\(name)"
  }

  /// A stable identity for annotation/list diffing.
  public var id: String {
    value
  }
}
