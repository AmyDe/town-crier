import Combine
import Foundation
import TownCrierDomain

/// Drives create/edit of a single watch zone with postcode geocoding and tier-based radius limits.
@MainActor
public final class WatchZoneEditorViewModel: ObservableObject, EntitlementGatingViewModel {
  @Published public var nameInput: String = ""
  @Published public var postcodeInput: String = ""
  @Published public var selectedRadiusMetres: Double = 1000
  @Published public var pushEnabled: Bool = true
  @Published public var emailInstantEnabled: Bool = true
  @Published public private(set) var geocodedCoordinate: Coordinate?
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  /// Drives the in-editor subscription upsell sheet. Non-nil while the gate is
  /// presented for the given entitlement; cleared when the sheet dismisses.
  @Published public var entitlementGate: Entitlement?

  var onSave: ((WatchZone) -> Void)?

  /// Invoked when a save fails because the user has hit their tier's watch-zone
  /// quota (tc-gpjk). The coordinator dismisses the editor and presents the
  /// subscription paywall — no inline error is shown for this case.
  public var onUpgradeRequired: (() -> Void)?

  public let isEditing: Bool

  private let geocoder: PostcodeGeocoder
  private let repository: WatchZoneRepository
  private let limits: WatchZoneLimits
  private let tier: SubscriptionTier
  private let existingId: WatchZoneId?

  public init(
    geocoder: PostcodeGeocoder,
    repository: WatchZoneRepository,
    tier: SubscriptionTier,
    editing zone: WatchZone? = nil
  ) {
    self.geocoder = geocoder
    self.repository = repository
    self.limits = WatchZoneLimits(tier: tier)
    self.tier = tier
    self.isEditing = zone != nil
    self.existingId = zone?.id

    if let zone {
      self.nameInput = zone.name
      self.selectedRadiusMetres = zone.radiusMetres
      self.geocodedCoordinate = zone.centre
      self.pushEnabled = zone.pushEnabled
      self.emailInstantEnabled = zone.emailInstantEnabled
    }
  }

  public var availableRadiusOptions: [Double] {
    limits.availableRadiusOptions
  }

  public var isPostcodeFieldVisible: Bool {
    !isEditing
  }

  public var maxRadiusMetres: Double {
    limits.maxRadiusMetres
  }

  /// Proactive UI gate exposing the session's tier to entitlement-aware controls
  /// (e.g. ``GatedToggle``) without a network round-trip.
  public var featureGate: FeatureGate {
    FeatureGate(tier: tier)
  }

  /// The entitlement that the per-zone instant push/email toggles are gated behind.
  ///
  /// Instant alerts (push and instant email) are paid-only; free accounts receive
  /// the weekly digest only and the server never delivers instant alerts to them
  /// (tc-bd6i). All paid entitlements travel together in ``EntitlementMap``, so any
  /// paid-only entitlement correctly distinguishes free (locked) from paid (open).
  public var instantAlertEntitlement: Entitlement {
    .statusChangeAlerts
  }

  /// The notifications section is always shown. For free accounts the instant
  /// push/email toggles render locked with an upgrade prompt via ``GatedToggle``;
  /// for Personal/Pro they are fully interactive (tc-bd6i).
  public var areNotificationTogglesVisible: Bool {
    true
  }

  /// Surfaces the in-editor subscription upsell when a free-tier user taps a locked
  /// instant-alert toggle. Routes through the entitlement gate so the editor stays
  /// open and the user can dismiss without losing their work.
  public func requestInstantAlertUpgrade() {
    entitlementGate = instantAlertEntitlement
  }

  /// Invoked when the user taps "View Plans" in the upsell sheet — hands off to the
  /// coordinator (via ``onUpgradeRequired``) to present the subscription screen.
  public func viewPlans() {
    onUpgradeRequired?()
  }

  /// Whether to surface the "this zone may produce lots of notifications" callout
  /// (tc-1zb7). Triggered at or above 2 km, the upper edge of the recommended
  /// "small zone" range — see `LargeRadiusWarningView`.
  public var showsLargeRadiusWarning: Bool {
    selectedRadiusMetres >= LargeRadiusWarning.thresholdMetres
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
  /// On a quota breach (`DomainError.insufficientEntitlement`) the editor routes
  /// to the subscription paywall via `onUpgradeRequired` and leaves `error`
  /// unset — the coordinator closes the sheet. All other failures set `error`
  /// so the inline error section is shown and the editor stays open.
  @discardableResult
  public func save() async -> Bool {
    guard let coordinate = geocodedCoordinate else { return false }
    error = nil

    let clampedRadius = limits.clampRadius(selectedRadiusMetres)

    do {
      let zone = try WatchZone(
        id: existingId ?? WatchZoneId(),
        name: nameInput,
        centre: coordinate,
        radiusMetres: clampedRadius,
        pushEnabled: pushEnabled,
        emailInstantEnabled: emailInstantEnabled
      )
      if isEditing {
        try await repository.update(zone)
      } else {
        try await repository.save(zone)
      }
      onSave?(zone)
      return true
    } catch DomainError.insufficientEntitlement {
      onUpgradeRequired?()
      return false
    } catch {
      handleError(error)
      return false
    }
  }
}
