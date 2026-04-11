#if DEBUG
  import Foundation
  import TownCrierDomain

  enum SampleData {
    static let camden = LocalAuthority(code: "CMD", name: "Camden")

    static let applications: [PlanningApplication] = {
      let calendar = Calendar.current
      let now = Date()

      func daysAgo(_ days: Int) -> Date {
        // swiftlint:disable:next force_unwrapping
        calendar.date(byAdding: .day, value: -days, to: now)!
      }

      return [
        PlanningApplication(
          id: PlanningApplicationId("app-001"),
          reference: ApplicationReference("2026/0142/P"),
          authority: camden,
          status: .undecided,
          receivedDate: daysAgo(3),
          description: "Erection of single-storey rear extension and associated landscaping",
          address: "14 Dartmouth Park Road, London NW5 1SU",
          location: try? Coordinate(latitude: 51.5615, longitude: -0.1378),
          portalUrl: URL(string: "https://planit.org.uk/planapplic/example1"),
          statusHistory: [
            StatusEvent(status: .undecided, date: daysAgo(3))
          ]
        ),
        PlanningApplication(
          id: PlanningApplicationId("app-002"),
          reference: ApplicationReference("2026/0098/F"),
          authority: camden,
          status: .approved,
          receivedDate: daysAgo(45),
          // swiftlint:disable:next line_length
          description:
            "Change of use from retail (Class E) to restaurant (Sui Generis) with extraction flue to rear",
          address: "87 Kentish Town Road, London NW1 8NY",
          location: try? Coordinate(latitude: 51.5475, longitude: -0.1420),
          portalUrl: URL(string: "https://planit.org.uk/planapplic/example2"),
          statusHistory: [
            StatusEvent(status: .undecided, date: daysAgo(45)),
            StatusEvent(status: .approved, date: daysAgo(5)),
          ]
        ),
        PlanningApplication(
          id: PlanningApplicationId("app-003"),
          reference: ApplicationReference("2026/0201/P"),
          authority: camden,
          status: .undecided,
          receivedDate: daysAgo(7),
          description:
            "Conversion of existing loft space including rear dormer window and two front rooflights",
          address: "31 Lady Margaret Road, London NW5 2NH",
          location: try? Coordinate(latitude: 51.5560, longitude: -0.1445),
          portalUrl: URL(string: "https://planit.org.uk/planapplic/example3"),
          statusHistory: [
            StatusEvent(status: .undecided, date: daysAgo(7))
          ]
        ),
        PlanningApplication(
          id: PlanningApplicationId("app-004"),
          reference: ApplicationReference("2025/4871/F"),
          authority: camden,
          status: .refused,
          receivedDate: daysAgo(90),
          description: "Demolition of existing garage and erection of two-storey dwelling house",
          address: "Rear of 5 Highgate West Hill, London N6 6BU",
          location: try? Coordinate(latitude: 51.5680, longitude: -0.1490),
          portalUrl: URL(string: "https://planit.org.uk/planapplic/example4"),
          statusHistory: [
            StatusEvent(status: .undecided, date: daysAgo(90)),
            StatusEvent(status: .refused, date: daysAgo(12)),
          ]
        ),
        PlanningApplication(
          id: PlanningApplicationId("app-005"),
          reference: ApplicationReference("2026/0055/L"),
          authority: camden,
          status: .approved,
          receivedDate: daysAgo(60),
          // swiftlint:disable:next line_length
          description:
            "Listed building consent for internal alterations including removal of non-original partition walls and installation of new kitchen",
          address: "12 Church Row, Hampstead, London NW3 6UP",
          location: try? Coordinate(latitude: 51.5575, longitude: -0.1775),
          portalUrl: URL(string: "https://planit.org.uk/planapplic/example5"),
          statusHistory: [
            StatusEvent(status: .undecided, date: daysAgo(60)),
            StatusEvent(status: .approved, date: daysAgo(8)),
          ]
        ),
        PlanningApplication(
          id: PlanningApplicationId("app-006"),
          reference: ApplicationReference("2026/0178/P"),
          authority: camden,
          status: .undecided,
          receivedDate: daysAgo(10),
          description:
            "Installation of replacement windows to front and rear elevations (conservation area)",
          address: "44 Grafton Terrace, London NW5 4JA",
          location: try? Coordinate(latitude: 51.5530, longitude: -0.1465),
          portalUrl: URL(string: "https://planit.org.uk/planapplic/example6"),
          statusHistory: [
            StatusEvent(status: .undecided, date: daysAgo(10))
          ]
        ),
        PlanningApplication(
          id: PlanningApplicationId("app-007"),
          reference: ApplicationReference("2026/0033/F"),
          authority: camden,
          status: .withdrawn,
          receivedDate: daysAgo(55),
          description: "Erection of roof terrace with glazed balustrade at third-floor level",
          address: "Unit 3, 199 Arlington Road, London NW1 7HA",
          location: try? Coordinate(latitude: 51.5390, longitude: -0.1465),
          portalUrl: URL(string: "https://planit.org.uk/planapplic/example7"),
          statusHistory: [
            StatusEvent(status: .undecided, date: daysAgo(55)),
            StatusEvent(status: .withdrawn, date: daysAgo(20)),
          ]
        ),
        PlanningApplication(
          id: PlanningApplicationId("app-008"),
          reference: ApplicationReference("2026/0210/P"),
          authority: camden,
          status: .undecided,
          receivedDate: daysAgo(2),
          // swiftlint:disable:next line_length
          description:
            "Construction of basement level beneath existing garden for use as home gym and cinema room",
          address: "8 Nassington Road, London NW3 2TY",
          location: try? Coordinate(latitude: 51.5635, longitude: -0.1540),
          portalUrl: URL(string: "https://planit.org.uk/planapplic/example8"),
          statusHistory: [
            StatusEvent(status: .undecided, date: daysAgo(2))
          ]
        ),
      ]
    }()

    static let watchZone: WatchZone = {
      // swiftlint:disable force_try
      let centre = try! Coordinate(latitude: 51.5550, longitude: -0.1450)
      let postcode = try! Postcode("NW5 1SU")
      let zone = try! WatchZone(postcode: postcode, centre: centre, radiusMetres: 2000)
      // swiftlint:enable force_try
      return zone
    }()
  }
#endif
