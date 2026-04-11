import Foundation
import TownCrierDomain

final class SpyZonePreferencesRepository: ZonePreferencesRepository, @unchecked Sendable {
  private(set) var fetchCalls: [String] = []
  var fetchResult: Result<ZoneNotificationPreferences, Error> = .success(
    ZoneNotificationPreferences(zoneId: "default")
  )

  func fetchPreferences(zoneId: String) async throws -> ZoneNotificationPreferences {
    fetchCalls.append(zoneId)
    return try fetchResult.get()
  }

  private(set) var updateCalls: [ZoneNotificationPreferences] = []
  var updateResult: Result<Void, Error> = .success(())

  func updatePreferences(_ preferences: ZoneNotificationPreferences) async throws {
    updateCalls.append(preferences)
    try updateResult.get()
  }
}
