import Foundation
import TownCrierDomain

/// Backs the anonymous browse flow's postcode entry screen (GH#868 Phase 3):
/// validates and geocodes the entered postcode via ``PostcodeGeocoder`` (the
/// device-side ``PostcodesIOGeocoder`` in production — never `/v1/geocode`,
/// which requires a session), persists the resolved
/// ``AnonymousBrowseState``, and hands it to ``AnonymousBrowseCoordinator``
/// via ``onResolved`` to advance to the map.
@MainActor
public final class AnonymousPostcodeEntryViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public var postcodeInput: String = ""
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?

  private let geocoder: PostcodeGeocoder
  private let stateRepository: AnonymousBrowseStateRepository

  /// Fired when the user taps "Back" — the coordinator returns to welcome.
  public var onBack: (() -> Void)?
  /// Fired once postcode entry has resolved and persisted a state — the
  /// coordinator builds the map and advances the flow.
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
        postcode: postcode, coordinate: coordinate, createdAt: Date())
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
