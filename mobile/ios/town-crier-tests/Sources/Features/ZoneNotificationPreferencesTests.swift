import Foundation
import Testing

@testable import TownCrierDomain

@Suite("ZoneNotificationPreferences")
struct ZoneNotificationPreferencesTests {

  @Test func defaultPreferences_allDisabledExceptNewApplications() {
    let prefs = ZoneNotificationPreferences(zoneId: "zone-001")

    #expect(prefs.zoneId == "zone-001")
    #expect(prefs.newApplications == true)
    #expect(prefs.statusChanges == false)
    #expect(prefs.decisionUpdates == false)
  }

  @Test func customPreferences_retainAllValues() {
    let prefs = ZoneNotificationPreferences(
      zoneId: "zone-002",
      newApplications: false,
      statusChanges: true,
      decisionUpdates: true
    )

    #expect(prefs.zoneId == "zone-002")
    #expect(prefs.newApplications == false)
    #expect(prefs.statusChanges == true)
    #expect(prefs.decisionUpdates == true)
  }

  @Test func equatable_sameValues_areEqual() {
    let a = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplications: true,
      statusChanges: false,
      decisionUpdates: true
    )
    let b = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplications: true,
      statusChanges: false,
      decisionUpdates: true
    )

    #expect(a == b)
  }

  @Test func equatable_differentValues_areNotEqual() {
    let a = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplications: true,
      statusChanges: false,
      decisionUpdates: false
    )
    let b = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplications: true,
      statusChanges: true,
      decisionUpdates: false
    )

    #expect(a != b)
  }
}
