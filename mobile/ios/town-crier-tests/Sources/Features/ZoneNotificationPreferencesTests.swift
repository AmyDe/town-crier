import Foundation
import Testing

@testable import TownCrierDomain

@Suite("ZoneNotificationPreferences")
struct ZoneNotificationPreferencesTests {

  @Test func defaultPreferences_allFourTogglesEnabled() {
    let prefs = ZoneNotificationPreferences(zoneId: "zone-001")

    #expect(prefs.zoneId == "zone-001")
    #expect(prefs.newApplicationPush == true)
    #expect(prefs.newApplicationEmail == true)
    #expect(prefs.decisionPush == true)
    #expect(prefs.decisionEmail == true)
  }

  @Test func customPreferences_retainAllValues() {
    let prefs = ZoneNotificationPreferences(
      zoneId: "zone-002",
      newApplicationPush: false,
      newApplicationEmail: true,
      decisionPush: false,
      decisionEmail: true
    )

    #expect(prefs.zoneId == "zone-002")
    #expect(prefs.newApplicationPush == false)
    #expect(prefs.newApplicationEmail == true)
    #expect(prefs.decisionPush == false)
    #expect(prefs.decisionEmail == true)
  }

  @Test func equatable_sameValues_areEqual() {
    let a = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplicationPush: true,
      newApplicationEmail: false,
      decisionPush: true,
      decisionEmail: false
    )
    let b = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplicationPush: true,
      newApplicationEmail: false,
      decisionPush: true,
      decisionEmail: false
    )

    #expect(a == b)
  }

  @Test func equatable_differentValues_areNotEqual() {
    let a = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplicationPush: true,
      newApplicationEmail: true,
      decisionPush: false,
      decisionEmail: true
    )
    let b = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplicationPush: true,
      newApplicationEmail: true,
      decisionPush: true,
      decisionEmail: true
    )

    #expect(a != b)
  }
}
