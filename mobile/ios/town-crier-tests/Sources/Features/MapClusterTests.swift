import Testing
import TownCrierDomain

@Suite("MapCluster")
struct MapClusterTests {
  @Test func singleMember_carriesMemberIdentityAndStatus() throws {
    let coordinate = try Coordinate(latitude: 51.5, longitude: -0.12)
    let member = PlanningApplicationId(authority: "42", name: "22/1234/FUL")

    let cluster = MapCluster(
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

    let cluster = MapCluster(
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
    let member = PlanningApplicationId(authority: "42", name: "22/1234/FUL")

    let cluster = MapCluster(
      coordinate: coordinate, count: 1, statusCounts: [.permitted: 1], member: member)

    #expect(cluster.id == member.value)
  }

  @Test func id_distinguishesMultiMemberCellsByCoordinate() throws {
    let first = MapCluster(
      coordinate: try Coordinate(latitude: 51.5, longitude: -0.12),
      count: 10,
      statusCounts: [.permitted: 10],
      member: nil)
    let second = MapCluster(
      coordinate: try Coordinate(latitude: 51.6, longitude: -0.12),
      count: 10,
      statusCounts: [.permitted: 10],
      member: nil)

    #expect(first.id != second.id)
  }

  // MARK: - Stacked (unsplittable) cells (GH#722)

  @Test func stacked_whenMultiMemberWithCarriedMembers() throws {
    let coordinate = try Coordinate(latitude: 51.5, longitude: -0.12)
    let members = [
      PlanningApplicationId(authority: "42", name: "22/1234/FUL"),
      PlanningApplicationId(authority: "42", name: "22/5678/FUL"),
    ]

    let cluster = MapCluster(
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

    let cluster = MapCluster(
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
    let member = PlanningApplicationId(authority: "42", name: "22/1234/FUL")

    let cluster = MapCluster(
      coordinate: coordinate, count: 1, statusCounts: [.permitted: 1], member: member)

    #expect(!cluster.isStacked)
  }
}
