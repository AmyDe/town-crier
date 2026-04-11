import Foundation
import TownCrierDomain

extension SavedApplication {
  static let rearExtension = SavedApplication(
    applicationUid: "BK/2026/0042",
    savedAt: Date(timeIntervalSince1970: 1_700_000_000),
    application: .pendingReview
  )

  static let changeOfUse = SavedApplication(
    applicationUid: "BK/2026/0099",
    savedAt: Date(timeIntervalSince1970: 1_700_100_000),
    application: .approved
  )
}
