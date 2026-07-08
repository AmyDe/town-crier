import Foundation
import TownCrierDomain

/// Creates or edits a single device-local zone (GH#879 Phase 4): name,
/// postcode entry geocoded client-side via `PostcodeGeocoder` — never
/// `/v1/geocode` — and a radius clamped to
/// `[DeviceLocalZone.minRadiusMetres, DeviceLocalZone.maxRadiusMetres]`.
///
/// Visual/interaction conventions mirror the authed `WatchZoneEditorViewModel`,
/// but this is a distinct, simpler type — no tier, no entitlement gating, no
/// per-zone notification toggles. Any alert affordance on a device-local zone
/// is a sign-up CTA handled by the list, not the editor.
@MainActor
public final class DeviceLocalZoneEditorViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public var nameInput: String = ""
  @Published public var postcodeInput: String = ""
  @Published public var selectedRadiusMetres: Double = 1000
  @Published public private(set) var geocodedCoordinate: Coordinate?
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  var onSave: ((DeviceLocalZone) -> Void)?

  /// Invoked when saving would exceed the on-device cap
  /// (`DomainError.deviceLocalZoneLimitReached`) — the list dismisses the
  /// editor and shows the sign-up CTA instead of an inline error.
  public var onRequestSignUp: (() -> Void)?

  public let isEditing: Bool
  public let minRadiusMetres = DeviceLocalZone.minRadiusMetres
  public let maxRadiusMetres = DeviceLocalZone.maxRadiusMetres

  private let geocoder: PostcodeGeocoder
  private let repository: DeviceLocalZoneRepository
  private let existingId: DeviceLocalZoneId?

  public init(
    geocoder: PostcodeGeocoder,
    repository: DeviceLocalZoneRepository,
    editing zone: DeviceLocalZone? = nil
  ) {
    self.geocoder = geocoder
    self.repository = repository
    self.isEditing = zone != nil
    self.existingId = zone?.id

    if let zone {
      self.nameInput = zone.name
      self.selectedRadiusMetres = zone.radiusMetres
      self.geocodedCoordinate = zone.centre
    }
  }

  public var isPostcodeFieldVisible: Bool {
    !isEditing
  }

  public func submitPostcode() async {
    isLoading = true
    error = nil

    let postcode: Postcode
    do {
      postcode = try Postcode(postcodeInput)
    } catch {
      handleError(error)
      isLoading = false
      return
    }

    do {
      geocodedCoordinate = try await geocoder.geocode(postcode)
      if nameInput.trimmingCharacters(in: .whitespaces).isEmpty {
        nameInput = postcode.value
      }
    } catch {
      handleError(error)
    }

    isLoading = false
  }

  /// Persists the zone. Returns `true` on success so the View dismisses only
  /// when the save actually succeeded.
  ///
  /// On a cap breach (`DomainError.deviceLocalZoneLimitReached`) routes to
  /// the sign-up CTA via `onRequestSignUp` and leaves `error` unset; all
  /// other failures set `error` so the inline error section is shown and the
  /// editor stays open.
  @discardableResult
  public func save() async -> Bool {
    guard let coordinate = geocodedCoordinate else { return false }
    error = nil

    do {
      let zone = try DeviceLocalZone(
        id: existingId ?? DeviceLocalZoneId(),
        name: nameInput,
        centre: coordinate,
        radiusMetres: selectedRadiusMetres
      )
      try repository.save(zone)
      onSave?(zone)
      return true
    } catch DomainError.deviceLocalZoneLimitReached {
      onRequestSignUp?()
      return false
    } catch {
      handleError(error)
      return false
    }
  }
}
