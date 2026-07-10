import Testing
import TownCrierDomain

/// Covers ``AnonymousMapCluster``/``AnonymousClusterMember`` — the anonymous
/// mirror of `MapCluster`/`PlanningApplicationId` (GH#924 Phase 2), kept as
/// distinct types so the anonymous surface never depends on the authenticated
/// one (see the GH#924 issue's Pre-Resolved Design Decisions).
@Suite("AnonymousMapCluster")
struct AnonymousMapClusterTests {
  @Test func singleMember_carriesMemberIdentityAndStatus() throws {
    let coordinate = try Coordinate(latitude: 51.5, longitude: -0.12)
    let member = AnonymousClusterMember(
      authority: "314", name: "Kingston/22/1234/FUL", authoritySlug: "kingston")

    let cluster = AnonymousMapCluster(
      coordinate: coordinate,
      count: 1,
      statusCounts: [.permitted: 1],
      member: member
    )

    #expect(cluster.isSingleMember)
    #expect(cluster.member == member)
    #expect(cluster.memberStatus == .permitted)
  }

  @Test func multiMember_hasNilMemberAndNilMemberStatus() throws {
    let coordinate = try Coordinate(latitude: 51.5, longitude: -0.12)

    let cluster = AnonymousMapCluster(
      coordinate: coordinate,
      count: 194,
      statusCounts: [.permitted: 120, .undecided: 60, .rejected: 14],
      member: nil
    )

    #expect(!cluster.isSingleMember)
    #expect(cluster.member == nil)
    #expect(cluster.memberStatus == nil)
  }

  @Test func id_isTheMemberValue_forSingleMemberCluster() throws {
    let coordinate = try Coordinate(latitude: 51.5, longitude: -0.12)
    let member = AnonymousClusterMember(
      authority: "314", name: "Kingston/22/1234/FUL", authoritySlug: "kingston")

    let cluster = AnonymousMapCluster(
      coordinate: coordinate, count: 1, statusCounts: [.permitted: 1], member: member)

    #expect(cluster.id == member.value)
  }

  @Test func id_distinguishesMultiMemberCellsByCoordinate() throws {
    let first = AnonymousMapCluster(
      coordinate: try Coordinate(latitude: 51.5, longitude: -0.12),
      count: 10,
      statusCounts: [.permitted: 10],
      member: nil)
    let second = AnonymousMapCluster(
      coordinate: try Coordinate(latitude: 51.6, longitude: -0.12),
      count: 10,
      statusCounts: [.permitted: 10],
      member: nil)

    #expect(first.id != second.id)
  }

  // MARK: - Stacked (unsplittable) cells

  @Test func stacked_whenMultiMemberWithCarriedMembers() throws {
    let coordinate = try Coordinate(latitude: 51.5, longitude: -0.12)
    let members = [
      AnonymousClusterMember(authority: "314", name: "Kingston/22/1234/FUL", authoritySlug: "kingston"),
      AnonymousClusterMember(authority: "314", name: "Kingston/22/5678/FUL", authoritySlug: "kingston"),
    ]

    let cluster = AnonymousMapCluster(
      coordinate: coordinate,
      count: 2,
      statusCounts: [.permitted: 1, .undecided: 1],
      member: nil,
      members: members
    )

    #expect(cluster.isStacked)
    #expect(cluster.members == members)
  }

  @Test func notStacked_whenMultiMemberWithoutCarriedMembers() throws {
    let coordinate = try Coordinate(latitude: 51.5, longitude: -0.12)

    let cluster = AnonymousMapCluster(
      coordinate: coordinate,
      count: 42,
      statusCounts: [.permitted: 42],
      member: nil
    )

    #expect(!cluster.isStacked)
    #expect(cluster.members.isEmpty)
  }

  @Test func notStacked_whenSingleMember() throws {
    let coordinate = try Coordinate(latitude: 51.5, longitude: -0.12)
    let member = AnonymousClusterMember(
      authority: "314", name: "Kingston/22/1234/FUL", authoritySlug: "kingston")

    let cluster = AnonymousMapCluster(
      coordinate: coordinate, count: 1, statusCounts: [.permitted: 1], member: member)

    #expect(!cluster.isStacked)
  }

  // MARK: - AnonymousClusterMember

  @Test func member_value_isTheSlashJoinedAuthorityAndName() {
    let member = AnonymousClusterMember(
      authority: "314", name: "Kingston/22/1234/FUL", authoritySlug: "kingston")

    #expect(member.value == "314/Kingston/22/1234/FUL")
  }
}
