import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

@MainActor
@Suite("ZonePreferencesViewModel")
struct ZonePreferencesViewModelTests {

  private var spyRepository: SpyZonePreferencesRepository
  private var sut: ZonePreferencesViewModel

  // Default: Personal tier (has statusChangeAlerts and decisionUpdateAlerts)
  init() {
    spyRepository = SpyZonePreferencesRepository()
    sut = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spyRepository,
      tier: .personal
    )
  }

  // MARK: - Initial state

  @Test func initialState_hasDefaultValues() {
    #expect(sut.newApplications == true)
    #expect(sut.statusChanges == false)
    #expect(sut.decisionUpdates == false)
    #expect(sut.isLoading == false)
    #expect(sut.error == nil)
    #expect(sut.entitlementGate == nil)
    #expect(sut.zoneName == "CB1 2AD")
  }

  @Test func featureGate_personalTier_hasNotificationEntitlements() {
    #expect(sut.featureGate.hasEntitlement(.statusChangeAlerts) == true)
    #expect(sut.featureGate.hasEntitlement(.decisionUpdateAlerts) == true)
  }

  @Test func featureGate_freeTier_lacksNotificationEntitlements() {
    let freeSut = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spyRepository,
      tier: .free
    )

    #expect(freeSut.featureGate.hasEntitlement(.statusChangeAlerts) == false)
    #expect(freeSut.featureGate.hasEntitlement(.decisionUpdateAlerts) == false)
  }

  // MARK: - Load preferences

  @Test func loadPreferences_populatesFieldsFromRepository() async {
    spyRepository.fetchResult = .success(
      ZoneNotificationPreferences(
        zoneId: "zone-001",
        newApplications: false,
        statusChanges: true,
        decisionUpdates: true
      )
    )

    await sut.loadPreferences()

    #expect(sut.newApplications == false)
    #expect(sut.statusChanges == true)
    #expect(sut.decisionUpdates == true)
    #expect(sut.isLoading == false)
    #expect(sut.error == nil)
    #expect(spyRepository.fetchCalls == ["zone-001"])
  }

  @Test func loadPreferences_networkError_setsError() async {
    spyRepository.fetchResult = .failure(DomainError.networkUnavailable)

    await sut.loadPreferences()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.isLoading == false)
  }

  // MARK: - Save preferences

  @Test func savePreferences_sendsCurrentStateToRepository() async {
    sut.newApplications = true
    sut.statusChanges = true
    sut.decisionUpdates = false

    await sut.savePreferences()

    #expect(spyRepository.updateCalls.count == 1)
    let saved = spyRepository.updateCalls[0]
    #expect(saved.zoneId == "zone-001")
    #expect(saved.newApplications == true)
    #expect(saved.statusChanges == true)
    #expect(saved.decisionUpdates == false)
  }

  @Test func savePreferences_networkError_setsError() async {
    spyRepository.updateResult = .failure(DomainError.networkUnavailable)

    await sut.savePreferences()

    #expect(sut.error == .networkUnavailable)
  }

  @Test func savePreferences_insufficientEntitlement_setsEntitlementGate() async {
    spyRepository.updateResult = .failure(
      DomainError.insufficientEntitlement(required: "statusChangeAlerts")
    )

    await sut.savePreferences()

    #expect(sut.entitlementGate == .statusChangeAlerts)
    #expect(sut.error == nil)
  }

  @Test func savePreferences_insufficientEntitlement_decisionUpdates_setsGate() async {
    spyRepository.updateResult = .failure(
      DomainError.insufficientEntitlement(required: "decisionUpdateAlerts")
    )

    await sut.savePreferences()

    #expect(sut.entitlementGate == .decisionUpdateAlerts)
    #expect(sut.error == nil)
  }

  // MARK: - Upsell sheet

  @Test func showUpgradeSheet_setsEntitlementGate() {
    sut.showUpgradeSheet(for: .statusChangeAlerts)

    #expect(sut.entitlementGate == .statusChangeAlerts)
  }
}
