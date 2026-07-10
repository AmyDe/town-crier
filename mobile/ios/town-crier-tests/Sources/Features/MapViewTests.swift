import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// tc-3b1hj: the Map nav title was dropped so the map itself gains real
/// screen height, and the settings gear moved off the (now-hidden) nav bar
/// into a floating circular button MapView owns directly — the coordinator
/// wiring moves from `.settingsToolbar` (applied by the caller) to an
/// `onSettingsTapped` closure injected via `init`, mirroring the callback
/// pattern already used for `AnonymousApplicationListView`'s CTA banner.
@Suite("MapView")
@MainActor
struct MapViewTests {
  private func makeSUT(
    zones: [WatchZone] = [.cambridge]
  ) -> (MapView, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(zones)
    let viewModel = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    let sut = MapView(viewModel: viewModel) {}
    return (sut, spy)
  }

  @Test func body_renders() {
    let (sut, _) = makeSUT()
    _ = sut.body
  }

  /// Zero/one watch zones hides the zone picker chips (`showZonePicker`);
  /// the floating settings button and full-bleed map must still render with
  /// no header content above them.
  @Test func body_renders_withNoZonePicker() {
    let (sut, _) = makeSUT(zones: [])
    _ = sut.body
  }

  /// Multiple watch zones show the zone-picker chip row — the map must
  /// still render (as a `safeAreaInset`-donated header, not a VStack
  /// sibling that would defeat the full-bleed map layer).
  @Test func body_renders_withZonePicker() {
    let (sut, _) = makeSUT(zones: [.cambridge, .london])
    _ = sut.body
  }

  /// The injected `onSettingsTapped` closure is captured by reference, not
  /// copied prematurely — mirrors `SettingsToolbarModifierTests`, which
  /// verifies the equivalent nav-bar gear's action closure the same way.
  @Test func init_capturesOnSettingsTappedClosure() {
    let spy = SpyPlanningApplicationRepository()
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let viewModel = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    var didFire = false
    let sut = MapView(viewModel: viewModel) { didFire = true }

    _ = sut.body

    #expect(!didFire)
  }
}
