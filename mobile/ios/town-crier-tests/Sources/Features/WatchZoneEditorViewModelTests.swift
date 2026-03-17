import Foundation
import Testing
@testable import TownCrierDomain
@testable import TownCrierPresentation

@MainActor
@Suite("WatchZoneEditorViewModel — create mode")
struct WatchZoneEditorCreateTests {
    private var spyGeocoder: SpyPostcodeGeocoder!
    private var spyRepository: SpyWatchZoneRepository!
    private var sut: WatchZoneEditorViewModel!

    init() {
        spyGeocoder = SpyPostcodeGeocoder()
        spyRepository = SpyWatchZoneRepository()
        sut = WatchZoneEditorViewModel(
            geocoder: spyGeocoder,
            repository: spyRepository,
            tier: .personal
        )
    }

    @Test func initialState_isCreate() {
        #expect(!sut.isEditing)
        #expect(sut.postcodeInput.isEmpty)
        #expect(sut.selectedRadiusMetres == 1000)
        #expect(sut.geocodedCoordinate == nil)
    }

    @Test func submitPostcode_geocodesAndSetsCoordinate() async {
        sut.postcodeInput = "CB1 2AD"
        spyGeocoder.geocodeResult = .success(.cambridge)

        await sut.submitPostcode()

        #expect(sut.geocodedCoordinate == .cambridge)
        #expect(spyGeocoder.geocodeCalls.count == 1)
        #expect(!sut.isLoading)
    }

    @Test func submitPostcode_invalidPostcode_setsError() async {
        sut.postcodeInput = "INVALID"

        await sut.submitPostcode()

        #expect(sut.error != nil)
        #expect(sut.geocodedCoordinate == nil)
    }

    @Test func submitPostcode_geocodingFails_setsError() async {
        sut.postcodeInput = "CB1 2AD"
        spyGeocoder.geocodeResult = .failure(DomainError.geocodingFailed("not found"))

        await sut.submitPostcode()

        #expect(sut.error == .geocodingFailed("not found"))
    }

    @Test func radiusOptions_reflectTierLimits() {
        let limits = WatchZoneLimits(tier: .personal)
        #expect(sut.availableRadiusOptions == limits.availableRadiusOptions)
    }

    @Test func radiusOptions_freeTier_capped() {
        let freeSut = WatchZoneEditorViewModel(
            geocoder: spyGeocoder,
            repository: spyRepository,
            tier: .free
        )
        let limits = WatchZoneLimits(tier: .free)
        #expect(freeSut.availableRadiusOptions == limits.availableRadiusOptions)
    }

    @Test func save_createsZoneAndCallsRepository() async {
        sut.postcodeInput = "CB1 2AD"
        spyGeocoder.geocodeResult = .success(.cambridge)
        await sut.submitPostcode()
        sut.selectedRadiusMetres = 2000

        await sut.save()

        #expect(spyRepository.saveCalls.count == 1)
        let saved = spyRepository.saveCalls.first
        #expect(saved?.postcode == .cambridge)
        #expect(saved?.centre == .cambridge)
        #expect(saved?.radiusMetres == 2000)
    }

    @Test func save_invokesOnSaveCallback() async {
        sut.postcodeInput = "CB1 2AD"
        spyGeocoder.geocodeResult = .success(.cambridge)
        await sut.submitPostcode()
        var savedZone: WatchZone?
        sut.onSave = { savedZone = $0 }

        await sut.save()

        #expect(savedZone != nil)
    }

    @Test func save_withoutGeocoding_doesNothing() async {
        await sut.save()

        #expect(spyRepository.saveCalls.isEmpty)
    }

    @Test func save_repositoryFails_setsError() async {
        sut.postcodeInput = "CB1 2AD"
        spyGeocoder.geocodeResult = .success(.cambridge)
        await sut.submitPostcode()
        spyRepository.saveResult = .failure(DomainError.networkUnavailable)

        await sut.save()

        #expect(sut.error == .networkUnavailable)
    }

    @Test func save_clampsRadiusToTierMax() async {
        let freeSut = WatchZoneEditorViewModel(
            geocoder: spyGeocoder,
            repository: spyRepository,
            tier: .free
        )
        freeSut.postcodeInput = "CB1 2AD"
        spyGeocoder.geocodeResult = .success(.cambridge)
        await freeSut.submitPostcode()
        freeSut.selectedRadiusMetres = 5000

        await freeSut.save()

        let saved = spyRepository.saveCalls.first
        #expect(saved?.radiusMetres == 2000)
    }
}

@MainActor
@Suite("WatchZoneEditorViewModel — edit mode")
struct WatchZoneEditorEditTests {
    private var spyGeocoder: SpyPostcodeGeocoder!
    private var spyRepository: SpyWatchZoneRepository!
    private var sut: WatchZoneEditorViewModel!

    init() {
        spyGeocoder = SpyPostcodeGeocoder()
        spyRepository = SpyWatchZoneRepository()
        sut = WatchZoneEditorViewModel(
            geocoder: spyGeocoder,
            repository: spyRepository,
            tier: .personal,
            editing: .cambridge
        )
    }

    @Test func initialState_populatedFromExistingZone() {
        #expect(sut.isEditing)
        #expect(sut.postcodeInput == "CB1 2AD")
        #expect(sut.selectedRadiusMetres == 2000)
        #expect(sut.geocodedCoordinate == .cambridge)
    }

    @Test func save_preservesExistingId() async {
        await sut.save()

        let saved = spyRepository.saveCalls.first
        #expect(saved?.id == WatchZoneId("zone-001"))
    }

    @Test func submitPostcode_updatesCoordinateForNewPostcode() async {
        sut.postcodeInput = "SW1A 1AA"
        let london = try! Coordinate(latitude: 51.5014, longitude: -0.1419)
        spyGeocoder.geocodeResult = .success(london)

        await sut.submitPostcode()

        #expect(sut.geocodedCoordinate == london)
    }
}
