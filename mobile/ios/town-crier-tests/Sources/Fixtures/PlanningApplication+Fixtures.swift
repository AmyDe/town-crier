import Foundation
import TownCrierDomain

extension PlanningApplication {
    static let pendingReview = PlanningApplication(
        id: PlanningApplicationId("APP-001"),
        reference: ApplicationReference("2026/0042"),
        authority: LocalAuthority(code: "CAM", name: "Cambridge"),
        status: .underReview,
        receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
        description: "Erection of two-storey rear extension",
        address: "12 Mill Road, Cambridge, CB1 2AD",
        location: try! Coordinate(latitude: 52.2043, longitude: 0.1243)
    )

    static let approved = PlanningApplication(
        id: PlanningApplicationId("APP-002"),
        reference: ApplicationReference("2026/0099"),
        authority: LocalAuthority(code: "CAM", name: "Cambridge"),
        status: .approved,
        receivedDate: Date(timeIntervalSince1970: 1_700_100_000),
        description: "Change of use from retail to residential",
        address: "45 High Street, Cambridge, CB2 1LA",
        location: try! Coordinate(latitude: 52.2053, longitude: 0.1218)
    )

    static let refused = PlanningApplication(
        id: PlanningApplicationId("APP-003"),
        reference: ApplicationReference("2026/0150"),
        authority: LocalAuthority(code: "CAM", name: "Cambridge"),
        status: .refused,
        receivedDate: Date(timeIntervalSince1970: 1_700_200_000),
        description: "Demolition of existing garage and erection of dwelling",
        address: "8 Park Terrace, Cambridge, CB1 1JH",
        location: try! Coordinate(latitude: 52.2010, longitude: 0.1300)
    )

    static let withdrawn = PlanningApplication(
        id: PlanningApplicationId("APP-004"),
        reference: ApplicationReference("2026/0200"),
        authority: LocalAuthority(code: "CAM", name: "Cambridge"),
        status: .withdrawn,
        receivedDate: Date(timeIntervalSince1970: 1_700_300_000),
        description: "Installation of solar panels on south-facing roof",
        address: "22 Trumpington Street, Cambridge, CB2 1QA",
        location: try! Coordinate(latitude: 52.1990, longitude: 0.1190)
    )

    static let withPortalUrl = PlanningApplication(
        id: PlanningApplicationId("APP-005"),
        reference: ApplicationReference("2026/0042"),
        authority: LocalAuthority(code: "CAM", name: "Cambridge"),
        status: .underReview,
        receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
        description: "Erection of two-storey rear extension",
        address: "12 Mill Road, Cambridge, CB1 2AD",
        location: try! Coordinate(latitude: 52.2043, longitude: 0.1243),
        portalUrl: URL(string: "https://planning.cambridge.gov.uk/2026/0042")
    )
}

extension LocalAuthority {
    static let cambridge = LocalAuthority(code: "CAM", name: "Cambridge")
}

extension Coordinate {
    static let cambridge = try! Coordinate(latitude: 52.2053, longitude: 0.1218)
}

extension WatchZone {
    static let cambridge = try! WatchZone(
        centre: .cambridge,
        radiusMetres: 2000
    )
}
