import Foundation
import TownCrierDomain

/// Backs the anonymous browse flow's postcode entry screen (GH#868 Phase 3):
/// validates and geocodes the entered postcode via ``PostcodeGeocoder`` (the
/// device-side ``PostcodesIOGeocoder`` in production — never `/v1/geocode`,
/// which requires a session), persists the resolved
/// ``AnonymousBrowseState``, and hands it to ``AnonymousBrowseCoordinator``
/// via ``onResolved`` to advance to the tab shell (GH#879 Phase 3).
@MainActor
public final class AnonymousPostcodeEntryViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public var postcodeInput: String = ""
  /// The monitoring radius chosen on this screen (GH#912 Phase 4), persisted
  /// into ``AnonymousBrowseState`` on submit and seeding both the first
  /// ``DeviceLocalZone`` and the anonymous map's fetch/drawn-circle radius —
  /// replacing the removed map-slider as the sole way to set the initial
  /// radius pre-signup. Defaults to the free tier's cap, matching the
  /// eventual real limit a fresh account would get.
  @Published public var selectedRadiusMetres: Double = 2000
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  public let minRadiusMetres: Double = 100
  /// The free tier's radius cap — the anonymous flow has no session to
  /// resolve a real subscription tier against, so it always bounds the
  /// picker to what a brand-new free account would get.
  public let maxRadiusMetres: Double = WatchZoneLimits(tier: .free).maxRadiusMetres

  private let geocoder: PostcodeGeocoder
  private let stateRepository: AnonymousBrowseStateRepository

  /// Fired when the user taps "Back" — the coordinator returns to welcome.
  public var onBack: (() -> Void)?
  /// Fired once postcode entry has resolved and persisted a state — the
  /// coordinator builds the map view model and advances the flow to the tab
  /// shell.
  public var onResolved: ((AnonymousBrowseState) -> Void)?

  public init(geocoder: PostcodeGeocoder, stateRepository: AnonymousBrowseStateRepository) {
    self.geocoder = geocoder
    self.stateRepository = stateRepository
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
      let coordinate = try await geocoder.geocode(postcode)
      let state = AnonymousBrowseState(
        postcode: postcode,
        coordinate: coordinate,
        radiusMetres: selectedRadiusMetres,
        createdAt: Date())
      stateRepository.save(state)
      onResolved?(state)
    } catch {
      handleError(error)
    }

    isLoading = false
  }

  public func goBack() {
    onBack?()
  }
}
