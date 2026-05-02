import Testing
import TownCrierDomain

@Suite("Entitlement")
struct EntitlementTests {
  @Test("all cases are defined")
  func allCasesDefined() {
    let allCases = Entitlement.allCases
    #expect(allCases.count == 3)
    #expect(allCases.contains(.statusChangeAlerts))
    #expect(allCases.contains(.decisionUpdateAlerts))
    #expect(allCases.contains(.hourlyDigestEmails))
  }

  @Test("displayName returns user-facing text for each case")
  func displayNames() {
    #expect(Entitlement.statusChangeAlerts.displayName == "Status Change Alerts")
    #expect(Entitlement.decisionUpdateAlerts.displayName == "Decision Update Alerts")
    #expect(Entitlement.hourlyDigestEmails.displayName == "Hourly Digest Emails")
  }

  @Test("featureDescription returns marketing copy for each case")
  func featureDescriptions() {
    for entitlement in Entitlement.allCases {
      let description = entitlement.featureDescription
      #expect(!description.isEmpty)
    }
  }

  @Test("conforms to Identifiable with rawValue as id")
  func identifiable() {
    let entitlement = Entitlement.statusChangeAlerts
    #expect(entitlement.id == entitlement.rawValue)
  }
}
