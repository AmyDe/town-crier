import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AnonymousPostcodeEntryViewModel")
@MainActor
struct AnonymousPostcodeEntryViewModelTests {
  private func makeSUT() -> (
    AnonymousPostcodeEntryViewModel, SpyPostcodeGeocoder, SpyAnonymousBrowseStateRepository
  ) {
    let geocoder = SpyPostcodeGeocoder()
    let stateRepository = SpyAnonymousBrowseStateRepository()
    let sut = AnonymousPostcodeEntryViewModel(geocoder: geocoder, stateRepository: stateRepository)
    return (sut, geocoder, stateRepository)
  }

  @Test func submitPostcode_geocodesAndPersistsState() async {
    let (sut, geocoder, stateRepository) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    var resolved: AnonymousBrowseState?
    sut.onResolved = { resolved = $0 }

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.count == 1)
    #expect(geocoder.geocodeCalls.first?.value == "CB1 2AD")
    #expect(stateRepository.saveCalls.count == 1)
    #expect(stateRepository.saveCalls.first?.coordinate == .cambridge)
    #expect(resolved != nil)
    #expect(resolved?.coordinate == .cambridge)
  }

  @Test func submitPostcode_invalidPostcode_setsErrorAndSkipsGeocode() async {
    let (sut, geocoder, stateRepository) = makeSUT()
    sut.postcodeInput = "INVALID"

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.isEmpty)
    #expect(stateRepository.saveCalls.isEmpty)
    #expect(sut.error != nil)
  }

  @Test func submitPostcode_geocodingFailure_setsErrorAndSkipsSave() async {
    let (sut, geocoder, stateRepository) = makeSUT()
    geocoder.geocodeResult = .failure(DomainError.geocodingFailed("CB1 2AD"))
    sut.postcodeInput = "CB1 2AD"

    await sut.submitPostcode()

    #expect(sut.error == .geocodingFailed("CB1 2AD"))
    #expect(stateRepository.saveCalls.isEmpty)
  }

  @Test func submitPostcode_setsIsLoadingFalseAfterCompletion() async {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"

    #expect(!sut.isLoading)
    await sut.submitPostcode()
    #expect(!sut.isLoading)
  }

  @Test func goBack_invokesOnBack() {
    let (sut, _, _) = makeSUT()
    var invoked = false
    sut.onBack = { invoked = true }

    sut.goBack()

    #expect(invoked)
  }

  // MARK: - Radius picker (GH#912 Phase 4)

  @Test func init_defaultsSelectedRadiusToFreeTierMax() {
    let (sut, _, _) = makeSUT()

    #expect(sut.selectedRadiusMetres == 2000)
    #expect(sut.maxRadiusMetres == 2000)
    #expect(sut.minRadiusMetres == 100)
  }

  @Test func submitPostcode_persistsStateWithTheSelectedRadius() async {
    let (sut, geocoder, stateRepository) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    sut.selectedRadiusMetres = 1500
    var resolved: AnonymousBrowseState?
    sut.onResolved = { resolved = $0 }

    await sut.submitPostcode()

    #expect(stateRepository.saveCalls.last?.radiusMetres == 1500)
    #expect(resolved?.radiusMetres == 1500)
  }

  // MARK: - Live preview (GH#931)

  @Test func refreshPreview_validPostcode_setsPreviewCoordinate() async {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"

    await sut.refreshPreview()

    #expect(geocoder.geocodeCalls.count == 1)
    #expect(sut.previewCoordinate == .cambridge)
  }

  @Test func refreshPreview_invalidInput_clearsPreviewWithoutGeocoding() async {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    await sut.refreshPreview()
    #expect(sut.previewCoordinate == .cambridge)

    sut.postcodeInput = "INVALID"
    await sut.refreshPreview()

    #expect(sut.previewCoordinate == nil)
    #expect(geocoder.geocodeCalls.count == 1)
  }

  @Test func refreshPreview_samePostcodeTwice_geocodesOnlyOnce() async {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"

    await sut.refreshPreview()
    await sut.refreshPreview()

    #expect(geocoder.geocodeCalls.count == 1)
    #expect(sut.previewCoordinate == .cambridge)
  }

  @Test func refreshPreview_geocodeFailure_clearsPreviewAndSetsNoError() async {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    await sut.refreshPreview()
    #expect(sut.previewCoordinate == .cambridge)

    geocoder.geocodeResult = .failure(DomainError.geocodingFailed("SW1A 1AA"))
    sut.postcodeInput = "SW1A 1AA"
    await sut.refreshPreview()

    #expect(sut.previewCoordinate == nil)
    #expect(sut.error == nil)
  }

  @Test func refreshPreview_doesNotChangeIsLoading() async {
    let (sut, geocoder, _) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"

    #expect(!sut.isLoading)
    await sut.refreshPreview()
    #expect(!sut.isLoading)
  }

  @Test func submitPostcode_afterSuccessfulPreview_reusesCoordinateWithoutSecondGeocode() async {
    let (sut, geocoder, stateRepository) = makeSUT()
    geocoder.geocodeResult = .success(.cambridge)
    sut.postcodeInput = "CB1 2AD"
    await sut.refreshPreview()
    var resolved: AnonymousBrowseState?
    sut.onResolved = { resolved = $0 }

    await sut.submitPostcode()

    #expect(geocoder.geocodeCalls.count == 1)
    #expect(stateRepository.saveCalls.count == 1)
    #expect(stateRepository.saveCalls.first?.coordinate == .cambridge)
    #expect(resolved?.coordinate == .cambridge)
    #expect(resolved?.radiusMetres == sut.selectedRadiusMetres)
  }
}
