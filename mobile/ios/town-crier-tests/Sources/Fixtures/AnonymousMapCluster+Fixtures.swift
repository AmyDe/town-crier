import TownCrierDomain

// swiftlint:disable force_try discouraged_default_parameter

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
  /// A multi-member amber bubble cell (no carried member; a tap zooms in).
  static func bubble(
    count: Int = 3,
    latitude: Double = 51.5,
    longitude: Double = -0.12
  ) -> AnonymousMapCluster {
    AnonymousMapCluster(
      coordinate: try! Coordinate(latitude: latitude, longitude: longitude),
      count: count,
      statusCounts: [.permitted: count],
      member: nil)
  }

  /// A single-member status-pin cell carrying the member's identity (a tap
  /// point-reads the full application by slug).
  static func single(
    member: AnonymousClusterMember,
    status: ApplicationStatus = .permitted,
    latitude: Double = 51.5,
    longitude: Double = -0.12
  ) -> AnonymousMapCluster {
    AnonymousMapCluster(
      coordinate: try! Coordinate(latitude: latitude, longitude: longitude),
      count: 1,
      statusCounts: [status: 1],
      member: member)
  }

  /// An unsplittable (coincident) multi-member cell carrying its stacked
  /// members' identities; a tap opens the disambiguation list.
  static func stacked(
    members: [AnonymousClusterMember],
    latitude: Double = 51.5,
    longitude: Double = -0.12
  ) -> AnonymousMapCluster {
    AnonymousMapCluster(
      coordinate: try! Coordinate(latitude: latitude, longitude: longitude),
      count: members.count,
      statusCounts: [.permitted: members.count],
      member: nil,
      members: members)
  }
}
