import Foundation
import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

@MainActor
@Suite("ZonePreferencesView integration")
struct ZonePreferencesViewTests {

  // These tests verify the ViewModel contract that the View depends on.
  // The View itself is a passive renderer of ViewModel state.

  @Test func viewModel_loadAndSave_roundTripsCorrectly() async {
    let spy = SpyZonePreferencesRepository()
    spy.fetchResult = .success(
      ZoneNotificationPreferences(
        zoneId: "zone-001",
        newApplicationPush: true,
        newApplicationEmail: false,
        decisionPush: true,
        decisionEmail: false
      )
    )
    let vm = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spy
    )

    // Load
    await vm.loadPreferences()
    #expect(vm.newApplicationPush == true)
    #expect(vm.newApplicationEmail == false)
    #expect(vm.decisionPush == true)
    #expect(vm.decisionEmail == false)

    // User toggles decisionEmail on
    vm.decisionEmail = true
    await vm.savePreferences()

    #expect(spy.updateCalls.count == 1)
    let saved = spy.updateCalls[0]
    #expect(saved.newApplicationPush == true)
    #expect(saved.newApplicationEmail == false)
    #expect(saved.decisionPush == true)
    #expect(saved.decisionEmail == true)
  }

  @Test func viewCanBeInstantiated() {
    let spy = SpyZonePreferencesRepository()
    let vm = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spy
    )

    // Verify the View can be constructed without crashing
    _ = ZonePreferencesView(viewModel: vm)
  }
}
