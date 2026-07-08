import Foundation
import TownCrierDomain

/// Drives the anonymous (pre-signup) Zones tab (GH#879 Phase 4): up to
/// ``DeviceLocalZone/maxZoneCount`` device-local areas with create/edit/
/// delete. Deliberately has no notion of a real quota/entitlement flow —
/// attempting to add a 4th zone, and tapping any per-row alert affordance,
/// both route straight to a sign-up CTA (``isSignUpCTAPresented``).
@MainActor
public final class DeviceLocalZoneListViewModel: ObservableObject {
  /// What the create/edit sheet should show. `Identifiable` so it binds
  /// directly to `.sheet(item:)`.
  public enum EditorTarget: Identifiable, Equatable {
    case new
    case edit(DeviceLocalZone)

    public var id: String {
      switch self {
      case .new: return "new"
      case .edit(let zone): return zone.id.value
      }
    }
  }

  @Published public private(set) var zones: [DeviceLocalZone] = []
  @Published public var editorTarget: EditorTarget?
  @Published public var isSignUpCTAPresented = false

  private let repository: DeviceLocalZoneRepository
  private let geocoder: PostcodeGeocoder

  /// Fired when the user confirms the sign-up CTA (cap reached, or a
  /// per-row alert affordance tapped). Wired by the coordinator to the same
  /// single sign-up/sign-in entry point every anonymous CTA uses.
  public var onRequestSignUp: (() -> Void)?

  public init(repository: DeviceLocalZoneRepository, geocoder: PostcodeGeocoder) {
    self.repository = repository
    self.geocoder = geocoder
  }

  public var canAddZone: Bool {
    zones.count < DeviceLocalZone.maxZoneCount
  }

  public func load() {
    zones = repository.loadAll()
  }

  /// The "+" toolbar action and the empty state's "Add Area" button. Opens
  /// the editor when there's still headroom; otherwise routes straight to
  /// the sign-up CTA — never opens the editor just to reject the save.
  public func addZoneTapped() {
    if canAddZone {
      editorTarget = .new
    } else {
      isSignUpCTAPresented = true
    }
  }

  public func editZone(_ zone: DeviceLocalZone) {
    editorTarget = .edit(zone)
  }

  public func deleteZone(_ zone: DeviceLocalZone) {
    repository.delete(zone.id)
    zones.removeAll { $0.id == zone.id }
  }

  /// Any alert/notification affordance on a zone row is a sign-up CTA —
  /// device-local zones never deliver alerts (GH#879 Phase 4).
  public func requestAlertsSignUp() {
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

  /// Builds the editor view model for `target`, wiring its callbacks back
  /// into this list: a successful save dismisses the editor and reloads the
  /// zone list from the repository; a cap breach dismisses the editor and
  /// shows the sign-up CTA instead of an inline error.
  public func makeEditorViewModel(for target: EditorTarget) -> DeviceLocalZoneEditorViewModel {
    let editingZone: DeviceLocalZone? = {
      if case .edit(let zone) = target { return zone }
      return nil
    }()
    let viewModel = DeviceLocalZoneEditorViewModel(
      geocoder: geocoder, repository: repository, editing: editingZone)
    viewModel.onSave = { [weak self] _ in
      self?.editorTarget = nil
      self?.load()
    }
    viewModel.onRequestSignUp = { [weak self] in
      self?.editorTarget = nil
      self?.isSignUpCTAPresented = true
    }
    return viewModel
  }
}
