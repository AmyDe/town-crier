import Foundation
import TownCrierDomain

/// Drives the per-zone notification preferences screen.
///
/// Exposes four per-channel toggles (push/email × new-application/decision) wired to the
/// ``ZonePreferencesRepository``. All four default to true; the free-tier downgrade is
/// applied server-side at dispatch time, so the UI shows the same controls to every tier.
@MainActor
public final class ZonePreferencesViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public var newApplicationPush: Bool = true
  @Published public var newApplicationEmail: Bool = true
  @Published public var decisionPush: Bool = true
  @Published public var decisionEmail: Bool = true
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  public let zoneName: String

  private let zoneId: String
  private let repository: ZonePreferencesRepository

  public init(
    zoneId: String,
    zoneName: String,
    repository: ZonePreferencesRepository
  ) {
    self.zoneId = zoneId
    self.zoneName = zoneName
    self.repository = repository
  }

  /// Loads the current preferences from the API.
  public func loadPreferences() async {
    isLoading = true
    error = nil

    do {
      let prefs = try await repository.fetchPreferences(zoneId: zoneId)
      newApplicationPush = prefs.newApplicationPush
      newApplicationEmail = prefs.newApplicationEmail
      decisionPush = prefs.decisionPush
      decisionEmail = prefs.decisionEmail
    } catch {
      handleError(error)
    }

    isLoading = false
  }

  /// Saves the current preferences to the API.
  public func savePreferences() async {
    error = nil

    let prefs = ZoneNotificationPreferences(
      zoneId: zoneId,
      newApplicationPush: newApplicationPush,
      newApplicationEmail: newApplicationEmail,
      decisionPush: decisionPush,
      decisionEmail: decisionEmail
    )

    do {
      try await repository.updatePreferences(prefs)
    } catch {
      handleError(error)
    }
  }
}
