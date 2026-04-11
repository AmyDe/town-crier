import Foundation
import Testing

@testable import TownCrierDomain

@Suite("ZonePreferencesRepository protocol")
struct ZonePreferencesRepositoryProtocolTests {

  @Test func spy_fetchPreferences_recordsCallAndReturnsConfigured() async throws {
    let spy = SpyZonePreferencesRepository()
    let expected = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplications: true,
      statusChanges: true,
      decisionUpdates: false
    )
    spy.fetchResult = .success(expected)

    let result = try await spy.fetchPreferences(zoneId: "zone-001")

    #expect(spy.fetchCalls == ["zone-001"])
    #expect(result == expected)
  }

  @Test func spy_updatePreferences_recordsCall() async throws {
    let spy = SpyZonePreferencesRepository()
    let prefs = ZoneNotificationPreferences(
      zoneId: "zone-001",
      newApplications: true,
      statusChanges: false,
      decisionUpdates: true
    )

    try await spy.updatePreferences(prefs)

    #expect(spy.updateCalls.count == 1)
    #expect(spy.updateCalls[0] == prefs)
  }

  @Test func spy_fetchPreferences_throwsConfiguredError() async {
    let spy = SpyZonePreferencesRepository()
    spy.fetchResult = .failure(DomainError.networkUnavailable)

    await #expect(throws: DomainError.networkUnavailable) {
      _ = try await spy.fetchPreferences(zoneId: "zone-001")
    }
  }

  @Test func spy_updatePreferences_throwsConfiguredError() async {
    let spy = SpyZonePreferencesRepository()
    spy.updateResult = .failure(DomainError.networkUnavailable)
    let prefs = ZoneNotificationPreferences(zoneId: "zone-001")

    await #expect(throws: DomainError.networkUnavailable) {
      try await spy.updatePreferences(prefs)
    }
  }
}
