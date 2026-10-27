import Foundation
import TownCrierDomain

// swiftlint:disable force_try

extension SavedApplication {
  static let rearExtension = SavedApplication(
    applicationUid: "BK/2026/0042",
    savedAt: Date(timeIntervalSince1970: 1_700_000_000),
    application: .pendingReview
  )

  static let changeOfUse = SavedApplication(
    applicationUid: "BK/2026/0099",
    savedAt: Date(timeIntervalSince1970: 1_700_100_000),
    application: .permitted
  )

  /// Builds a SavedApplication with a denormalised PlanningApplication payload
  /// matching the supplied uid/status.
  static func fixture(
    uid: String,
    savedAt: Date = Date(timeIntervalSince1970: 1_700_000_000),
    status: ApplicationStatus = .undecided
  ) -> SavedApplication {
    // uid doubles as the PlanningApplicationId name; authority is "TEST".
    SavedApplication(
      applicationUid: uid,
      savedAt: savedAt,
      application: PlanningApplication(
        id: PlanningApplicationId(authority: "TEST", name: uid),
        reference: ApplicationReference("FIX/\(uid)"),
        authority: .cambridge,
        status: status,
        receivedDate: savedAt,
        description: "Fixture for \(uid)",
        address: "1 Fixture Lane, Cambridge",
        location: try! Coordinate(latitude: 52.2, longitude: 0.12)
      )
    )
  }
}

// swiftlint:enable force_try
