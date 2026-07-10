import TownCrierDomain

// swiftlint:disable force_try

extension AnonymousClusterMember {
  static let kingstonOne = AnonymousClusterMember(
    authority: "314", name: "Kingston/25/00001/FUL", authoritySlug: "kingston")

  static let kingstonTwo = AnonymousClusterMember(
    authority: "314", name: "Kingston/25/00002/FUL", authoritySlug: "kingston")

  static let kingstonThree = AnonymousClusterMember(
    authority: "314", name: "Kingston/25/00003/FUL", authoritySlug: "kingston")

  /// A member with no resolved slug — the practically-unreachable resolver-miss
  /// case; a tap on a cluster carrying this should be ignored silently.
  static let missingSlug = AnonymousClusterMember(
    authority: "314", name: "Kingston/25/00004/FUL", authoritySlug: "")
}

extension AnonymousMapCluster {
  /// The fixed coordinate every factory below places its cell at — no test
  /// call site varies it, so it's a constant rather than a defaulted
  /// parameter (`discouraged_default_parameter`; CI's older SwiftLint
  /// doesn't recognise that rule at all, so disabling it locally is itself
  /// an error there — see tc-2wu29 PR review. Dropping the parameter avoids
  /// the drift entirely rather than fighting over a disable comment).
  private static let testCoordinate = try! Coordinate(latitude: 51.5, longitude: -0.12)

  /// A multi-member amber bubble cell (no carried member; a tap zooms in).
  static func bubble(count: Int) -> AnonymousMapCluster {
    AnonymousMapCluster(
      coordinate: testCoordinate,
      count: count,
      statusCounts: [.permitted: count],
      member: nil)
  }

  /// A single-member status-pin cell carrying the member's identity (a tap
  /// point-reads the full application by slug).
  static func single(member: AnonymousClusterMember) -> AnonymousMapCluster {
    AnonymousMapCluster(
      coordinate: testCoordinate,
      count: 1,
      statusCounts: [.permitted: 1],
      member: member)
  }

  /// An unsplittable (coincident) multi-member cell carrying its stacked
  /// members' identities; a tap opens the disambiguation list.
  static func stacked(members: [AnonymousClusterMember]) -> AnonymousMapCluster {
    AnonymousMapCluster(
      coordinate: testCoordinate,
      count: members.count,
      statusCounts: [.permitted: members.count],
      member: nil,
      members: members)
  }
}
