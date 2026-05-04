import Combine
import Foundation
import TownCrierDomain

/// Drives create/edit of a single watch zone with postcode geocoding and tier-based radius limits.
@MainActor
public final class WatchZoneEditorViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public var nameInput: String = ""
  @Published public var postcodeInput: String = ""
  @Published public var selectedRadiusMetres: Double = 1000
  @Published public var pushEnabled: Bool = true
  @Published public var emailInstantEnabled: Bool = true
  @Published public private(set) var geocodedCoordinate: Coordinate?
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  var onSave: ((WatchZone) -> Void)?

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

  /// Per-zone notification toggles are gated to Personal/Pro tiers only (tc-kh1s).
  ///
  /// Free users do not see these controls — the polling fan-out skips push and
  /// instant email for them regardless of any flag values.
  public var areNotificationTogglesVisible: Bool {
    tier != .free
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

  public func save() async {
    guard let coordinate = geocodedCoordinate else { return }
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
    } catch {
      handleError(error)
    }
  }
}
