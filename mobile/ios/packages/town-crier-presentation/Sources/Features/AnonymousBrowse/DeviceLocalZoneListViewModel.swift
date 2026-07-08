import Foundation
import TownCrierDomain

/// Drives the anonymous (pre-signup) Zones tab (GH#888): exactly one
/// editable device-local area (``DeviceLocalZone/maxZoneCount`` == 1),
/// seeded from the onboarding postcode. No add, no delete — GH#888 reverses
/// GH#879 Phase 4's 3-zone cap, which made signing up a net loss of areas.
/// Tapping the zone still opens the editor; any per-row alert affordance
/// still routes straight to a sign-up CTA (``isSignUpCTAPresented``).
@MainActor
public final class DeviceLocalZoneListViewModel: ObservableObject {
  @Published public private(set) var zones: [DeviceLocalZone] = []
  /// The zone currently open in the editor sheet. `DeviceLocalZone` is
  /// already `Identifiable`, so this binds directly to `.sheet(item:)`
  /// without a separate wrapper enum — unlike GH#879 Phase 4, there is no
  /// "new" case any more since the add path is gone.
  @Published public var editorTarget: DeviceLocalZone?
  @Published public var isSignUpCTAPresented = false

  private let repository: DeviceLocalZoneRepository
  private let geocoder: PostcodeGeocoder

  /// Fired when the user confirms the sign-up CTA (a per-row alert
  /// affordance, or the persistent sign-up pitch below the zone). Wired by
  /// the coordinator to the same single sign-up/sign-in entry point every
  /// anonymous CTA uses.
  public var onRequestSignUp: (() -> Void)?

  /// Fired with the saved zone after a successful editor save (GH#888), so
  /// the coordinator can propagate the edit live to the Map tab
  /// (`AnonymousMapViewModel.updateActiveZone(_:)`) and the Applications
  /// list — mirrors ``AnonymousApplicationListViewModel/onActiveZoneChanged``.
  public var onZonesChanged: ((DeviceLocalZone) -> Void)?

  public init(repository: DeviceLocalZoneRepository, geocoder: PostcodeGeocoder) {
    self.repository = repository
    self.geocoder = geocoder
  }

  public func load() {
    zones = repository.loadAll()
  }

  public func editZone(_ zone: DeviceLocalZone) {
    editorTarget = zone
  }

  /// Any alert/notification affordance on the zone row is a sign-up CTA —
  /// device-local zones never deliver alerts.
  public func requestAlertsSignUp() {
    isSignUpCTAPresented = true
  }

  /// The persistent "want more areas or alerts?" pitch below the zone
  /// (GH#888) — with the cap at one, this is the only remaining route to
  /// another area.
  public func requestSignUpFromPitch() {
    isSignUpCTAPresented = true
  }

  public func dismissSignUpCTA() {
    isSignUpCTAPresented = false
  }

  public func confirmSignUp() {
    isSignUpCTAPresented = false
    onRequestSignUp?()
  }

  public func dismissEditor() {
    editorTarget = nil
  }

  /// Builds the editor view model for `zone`. A successful save dismisses
  /// the editor, reloads the zone list from the repository, and notifies
  /// ``onZonesChanged`` with the saved zone; a cap breach (defensive only —
  /// the UI no longer offers a create path) dismisses the editor and shows
  /// the sign-up CTA instead of an inline error.
  public func makeEditorViewModel(for zone: DeviceLocalZone) -> DeviceLocalZoneEditorViewModel {
    let viewModel = DeviceLocalZoneEditorViewModel(
      geocoder: geocoder, repository: repository, editing: zone)
    viewModel.onSave = { [weak self] saved in
      self?.editorTarget = nil
      self?.load()
      self?.onZonesChanged?(saved)
    }
    viewModel.onRequestSignUp = { [weak self] in
      self?.editorTarget = nil
      self?.isSignUpCTAPresented = true
    }
    return viewModel
  }
}
