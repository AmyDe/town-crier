import Foundation
import TownCrierDomain

/// Edits the anonymous user's single device-local zone; `editing` is a
/// required, already-geocoded zone. Postcode entry is geocoded
/// client-side via `PostcodeGeocoder` — never `/v1/geocode` — and radius is
/// clamped to `[DeviceLocalZone.minRadiusMetres, DeviceLocalZone.maxRadiusMetres]`.
///
/// The postcode field is always reachable, even though the zone already has
/// a coordinate: it's the only way to correct a mistyped onboarding postcode,
/// since there is no add or delete path.
///
/// Visual/interaction conventions mirror the authed `WatchZoneEditorViewModel`,
/// but this is a distinct, simpler type — no tier, no entitlement gating, no
/// per-zone notification toggles. Any alert affordance on a device-local zone
/// is a sign-up CTA handled by the list, not the editor.
@MainActor
public final class DeviceLocalZoneEditorViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public var nameInput: String
  @Published public var postcodeInput: String = ""
  @Published public var selectedRadiusMetres: Double
  /// The zone's coordinate — seeded from the zone being edited, and updated
  /// by a successful ``submitPostcode()``. Never nil.
  @Published public private(set) var geocodedCoordinate: Coordinate
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  var onSave: ((DeviceLocalZone) -> Void)?

  /// Invoked when saving would exceed the on-device cap
  /// (`DomainError.deviceLocalZoneLimitReached`) — the list dismisses the
  /// editor and shows the sign-up CTA instead of an inline error. Defensive
  /// only: this editor only ever edits the zone already occupying
  /// the cap, never creates a new one.
  public var onRequestSignUp: (() -> Void)?

  public let minRadiusMetres = DeviceLocalZone.minRadiusMetres
  public let maxRadiusMetres = DeviceLocalZone.maxRadiusMetres

  private let geocoder: PostcodeGeocoder
  private let repository: DeviceLocalZoneRepository
  private let existingId: DeviceLocalZoneId

  public init(
    geocoder: PostcodeGeocoder,
    repository: DeviceLocalZoneRepository,
    editing zone: DeviceLocalZone
  ) {
    self.geocoder = geocoder
    self.repository = repository
    self.existingId = zone.id
    self.nameInput = zone.name
    self.selectedRadiusMetres = zone.radiusMetres
    self.geocodedCoordinate = zone.centre
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
    error = nil

    do {
      let zone = try DeviceLocalZone(
        id: existingId,
        name: nameInput,
        centre: geocodedCoordinate,
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
