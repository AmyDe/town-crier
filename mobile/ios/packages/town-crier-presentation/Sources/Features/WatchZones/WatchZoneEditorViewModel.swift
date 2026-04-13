import Combine
import Foundation
import TownCrierDomain

/// Drives create/edit of a single watch zone with postcode geocoding and tier-based radius limits.
@MainActor
public final class WatchZoneEditorViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public var postcodeInput: String = ""
  @Published public var selectedRadiusMetres: Double = 1000
  @Published public private(set) var geocodedCoordinate: Coordinate?
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  var onSave: ((WatchZone) -> Void)?

  public let isEditing: Bool

  private let geocoder: PostcodeGeocoder
  private let repository: WatchZoneRepository
  private let limits: WatchZoneLimits
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
    self.isEditing = zone != nil
    self.existingId = zone?.id

    if let zone {
      self.postcodeInput = zone.name
      self.selectedRadiusMetres = zone.radiusMetres
      self.geocodedCoordinate = zone.centre
    }
  }

  public var availableRadiusOptions: [Double] {
    limits.availableRadiusOptions
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
    } catch {
      handleError(error)
    }

    isLoading = false
  }

  public func save() async {
    guard let coordinate = geocodedCoordinate else { return }
    error = nil

    // Validate the postcode input before saving
    let postcode: Postcode
    do {
      postcode = try Postcode(postcodeInput)
    } catch {
      self.error = .invalidPostcode(postcodeInput)
      return
    }

    let clampedRadius = limits.clampRadius(selectedRadiusMetres)

    do {
      let zone = try WatchZone(
        id: existingId ?? WatchZoneId(),
        name: postcode.value,
        centre: coordinate,
        radiusMetres: clampedRadius
      )
      try await repository.save(zone)
      onSave?(zone)
    } catch {
      handleError(error)
    }
  }
}
