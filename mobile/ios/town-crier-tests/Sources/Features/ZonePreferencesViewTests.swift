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
        newApplications: true,
        statusChanges: true,
        decisionUpdates: false
      )
    )
    let vm = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spy,
      tier: .personal
    )

    // Load
    await vm.loadPreferences()
    #expect(vm.newApplications == true)
    #expect(vm.statusChanges == true)
    #expect(vm.decisionUpdates == false)

    // User toggles decisionUpdates on
    vm.decisionUpdates = true
    await vm.savePreferences()

    #expect(spy.updateCalls.count == 1)
    let saved = spy.updateCalls[0]
    #expect(saved.decisionUpdates == true)
    #expect(saved.statusChanges == true)
    #expect(saved.newApplications == true)
  }

  @Test func viewModel_freeTier_gatedTogglesShowUpgradeBadge() {
    let spy = SpyZonePreferencesRepository()
    let vm = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spy,
      tier: .free
    )

    // Free tier: gated toggles should show upgrade badge
    #expect(vm.featureGate.shouldShowUpgradeBadge(for: .statusChangeAlerts) == true)
    #expect(vm.featureGate.shouldShowUpgradeBadge(for: .decisionUpdateAlerts) == true)
  }

  @Test func viewModel_proTier_allTogglesEnabled() {
    let spy = SpyZonePreferencesRepository()
    let vm = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spy,
      tier: .pro
    )

    #expect(vm.featureGate.hasEntitlement(.statusChangeAlerts) == true)
    #expect(vm.featureGate.hasEntitlement(.decisionUpdateAlerts) == true)
  }

  @Test func viewCanBeInstantiated() {
    let spy = SpyZonePreferencesRepository()
    let vm = ZonePreferencesViewModel(
      zoneId: "zone-001",
      zoneName: "CB1 2AD",
      repository: spy,
      tier: .personal
    )

    // Verify the View can be constructed without crashing
    _ = ZonePreferencesView(viewModel: vm)
  }
}
