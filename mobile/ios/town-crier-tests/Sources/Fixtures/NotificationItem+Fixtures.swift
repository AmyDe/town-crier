import Foundation
import TownCrierDomain

extension NotificationItem {
  static let rearExtension = NotificationItem(
    applicationName: "Rear extension at 12 Mill Road",
    applicationAddress: "12 Mill Road, Cambridge, CB1 2AD",
    applicationDescription: "Erection of two-storey rear extension",
    applicationType: "Full Planning Application",
    authorityId: 123,
    createdAt: Date(timeIntervalSince1970: 1_712_000_000),
    eventType: "NewApplication",
    decision: nil,
    sources: "Zone"
  )

  static let changeOfUse = NotificationItem(
    applicationName: "Change of use at 45 High Street",
    applicationAddress: "45 High Street, Cambridge, CB2 1LA",
    applicationDescription: "Change of use from retail to residential",
    applicationType: "Change of Use",
    authorityId: 123,
    createdAt: Date(timeIntervalSince1970: 1_712_100_000),
    eventType: "NewApplication",
    decision: nil,
    sources: "Zone"
  )

  static let solarPanels = NotificationItem(
    applicationName: "Solar panels at 22 Trumpington Street",
    applicationAddress: "22 Trumpington Street, Cambridge, CB2 1QA",
    applicationDescription: "Installation of solar panels on south-facing roof",
    applicationType: "Householder",
    authorityId: 456,
    createdAt: Date(timeIntervalSince1970: 1_712_200_000),
    eventType: "NewApplication",
    decision: nil,
    sources: "Zone"
  )

  static let permittedDecision = NotificationItem(
    applicationName: "Decision: Rear extension at 12 Mill Road",
    applicationAddress: "12 Mill Road, Cambridge, CB1 2AD",
    applicationDescription: "Erection of two-storey rear extension",
    applicationType: "Full Planning Application",
    authorityId: 123,
    createdAt: Date(timeIntervalSince1970: 1_712_300_000),
    eventType: "DecisionUpdate",
    decision: "Permitted",
    sources: "Zone, Saved"
  )

  static let unknownDecision = NotificationItem(
    applicationName: "Decision: Unknown vocab",
    applicationAddress: "1 Anywhere",
    applicationDescription: "Some change",
    applicationType: "Type",
    authorityId: 1,
    createdAt: Date(timeIntervalSince1970: 1_712_400_000),
    eventType: "DecisionUpdate",
    decision: "SomethingWeird",
    sources: "Zone"
  )
}
