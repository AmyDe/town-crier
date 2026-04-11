import Foundation
import TownCrierDomain

extension NotificationItem {
  static let rearExtension = NotificationItem(
    applicationName: "Rear extension at 12 Mill Road",
    applicationAddress: "12 Mill Road, Cambridge, CB1 2AD",
    applicationDescription: "Erection of two-storey rear extension",
    applicationType: "Full Planning Application",
    authorityId: 123,
    createdAt: Date(timeIntervalSince1970: 1_712_000_000)
  )

  static let changeOfUse = NotificationItem(
    applicationName: "Change of use at 45 High Street",
    applicationAddress: "45 High Street, Cambridge, CB2 1LA",
    applicationDescription: "Change of use from retail to residential",
    applicationType: "Change of Use",
    authorityId: 123,
    createdAt: Date(timeIntervalSince1970: 1_712_100_000)
  )

  static let solarPanels = NotificationItem(
    applicationName: "Solar panels at 22 Trumpington Street",
    applicationAddress: "22 Trumpington Street, Cambridge, CB2 1QA",
    applicationDescription: "Installation of solar panels on south-facing roof",
    applicationType: "Householder",
    authorityId: 456,
    createdAt: Date(timeIntervalSince1970: 1_712_200_000)
  )
}
