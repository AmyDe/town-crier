import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

@MainActor
@Suite("ZonePreferencesViewModel")
struct ZonePreferencesViewModelTests {

  private var spyRepository: SpyZonePreferencesRepository
  private var sut: ZonePreferencesViewModel

  init() {
    spyRepository = SpyZonePreferencesRepository()
    sut = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spyRepository
    )
  }

  // MARK: - Initial state

  @Test func initialState_allTogglesDefaultOn() {
    #expect(sut.newApplicationPush == true)
    #expect(sut.newApplicationEmail == true)
    #expect(sut.decisionPush == true)
    #expect(sut.decisionEmail == true)
    #expect(sut.isLoading == false)
    #expect(sut.error == nil)
    #expect(sut.zoneName == "CB1 2AD")
  }

  // MARK: - Load preferences

  @Test func loadPreferences_populatesFieldsFromRepository() async {
    spyRepository.fetchResult = .success(
      ZoneNotificationPreferences(
        zoneId: "zone-001",
        newApplicationPush: false,
        newApplicationEmail: true,
        decisionPush: false,
        decisionEmail: true
      )
    )

    await sut.loadPreferences()

    #expect(sut.newApplicationPush == false)
    #expect(sut.newApplicationEmail == true)
    #expect(sut.decisionPush == false)
    #expect(sut.decisionEmail == true)
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
    sut.newApplicationPush = true
    sut.newApplicationEmail = false
    sut.decisionPush = true
    sut.decisionEmail = false

    await sut.savePreferences()

    #expect(spyRepository.updateCalls.count == 1)
    let saved = spyRepository.updateCalls[0]
    #expect(saved.zoneId == "zone-001")
    #expect(saved.newApplicationPush == true)
    #expect(saved.newApplicationEmail == false)
    #expect(saved.decisionPush == true)
    #expect(saved.decisionEmail == false)
  }

  @Test func savePreferences_networkError_setsError() async {
    spyRepository.updateResult = .failure(DomainError.networkUnavailable)

    await sut.savePreferences()

    #expect(sut.error == .networkUnavailable)
  }
}
