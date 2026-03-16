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
        address: "12 Mill Road, Cambridge, CB1 2AD"
    )

    static let approved = PlanningApplication(
        id: PlanningApplicationId("APP-002"),
        reference: ApplicationReference("2026/0099"),
        authority: LocalAuthority(code: "CAM", name: "Cambridge"),
        status: .approved,
        receivedDate: Date(timeIntervalSince1970: 1_700_100_000),
        description: "Change of use from retail to residential",
        address: "45 High Street, Cambridge, CB2 1LA"
    )
}

extension LocalAuthority {
    static let cambridge = LocalAuthority(code: "CAM", name: "Cambridge")
}
