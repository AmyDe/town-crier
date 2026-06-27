import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("MapViewModel")
@MainActor
struct MapViewModelTests {
  private func makeSUT(
    applications: [PlanningApplication] = [],
    watchZones: [WatchZone] = [.cambridge]
  ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(applications)
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(watchZones)
    let vm = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    return (vm, spy, watchZoneSpy)
  }

  // MARK: - Loading

  @Test func loadApplications_populatesAnnotations() async {
    let apps = [PlanningApplication.pendingReview, .permitted, .rejected, .withdrawn]
    let (sut, _, _) = makeSUT(applications: apps)

    await sut.loadApplications()

    #expect(sut.annotations.count == 4)
  }

  @Test func loadApplications_setsIsLoadingDuringFetch() async {
    let (sut, _, _) = makeSUT()

    #expect(!sut.isLoading)
    await sut.loadApplications()
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_setsErrorOnFailure() async {
    let (sut, spy, _) = makeSUT()
    // drainAllPages calls fetchApplicationsPage; set the paged error path.
    spy.fetchApplicationsPageError = DomainError.networkUnavailable

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.annotations.isEmpty)
  }

  // MARK: - Annotations

  @Test func annotations_haveCorrectStatus() async {
    let apps: [PlanningApplication] = [.pendingReview, .permitted, .rejected, .withdrawn]
    let (sut, _, _) = makeSUT(applications: apps)

    await sut.loadApplications()

    let pending = sut.annotations.first { $0.applicationId == PlanningApplication.pendingReview.id }
    let permitted = sut.annotations.first { $0.applicationId == PlanningApplication.permitted.id }
    let rejected = sut.annotations.first { $0.applicationId == PlanningApplication.rejected.id }
    let withdrawn = sut.annotations.first { $0.applicationId == PlanningApplication.withdrawn.id }

    #expect(pending?.status == .undecided)
    #expect(permitted?.status == .permitted)
    #expect(rejected?.status == .rejected)
    #expect(withdrawn?.status == .withdrawn)
  }

  @Test func annotations_onlyIncludeApplicationsWithLocations() async {
    let noLocation = PlanningApplication(
      id: PlanningApplicationId(authority: "CAM", name: "APP-NO-LOC"),
      reference: ApplicationReference("2026/0300"),
      authority: .cambridge,
      status: .undecided,
      receivedDate: Date(timeIntervalSince1970: 1_700_000_000),
      description: "No location",
      address: "Unknown",
      location: nil
    )
    let (sut, _, _) = makeSUT(applications: [.pendingReview, noLocation])

    await sut.loadApplications()

    #expect(sut.annotations.count == 1)
    #expect(sut.annotations.first?.applicationId == PlanningApplication.pendingReview.id)
  }

  // MARK: - Watch zone

  @Test func loadApplications_setsCentreFromWatchZone() async throws {
    let zone = try WatchZone(
      postcode: Postcode("SW1A 1AA"),
      centre: Coordinate(latitude: 51.5, longitude: -0.1),
      radiusMetres: 3000
    )
    let (sut, _, _) = makeSUT(watchZones: [zone])

    await sut.loadApplications()

    #expect(sut.centreLat == 51.5)
    #expect(sut.centreLon == -0.1)
    #expect(sut.radiusMetres == 3000)
  }

  @Test func loadApplications_setsError_whenWatchZoneFetchFails() async {
    let (sut, _, watchZoneSpy) = makeSUT()
    watchZoneSpy.loadAllResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    // Keeps London defaults when zone fetch fails
    #expect(sut.centreLat == 51.5074)
    #expect(sut.centreLon == -0.1278)
    #expect(sut.radiusMetres == 2000)
  }

  @Test func loadApplications_fetchesWatchZone() async {
    let (sut, _, watchZoneSpy) = makeSUT()

    await sut.loadApplications()

    #expect(watchZoneSpy.loadAllCallCount == 1)
  }

  // MARK: - Selection

  @Test func selectAnnotation_setsSelectedApplication() async {
    let apps = [PlanningApplication.pendingReview]
    let (sut, _, _) = makeSUT(applications: apps)

    await sut.loadApplications()
    sut.selectApplication(PlanningApplication.pendingReview.id)

    #expect(sut.selectedApplication?.id == PlanningApplication.pendingReview.id)
  }

  @Test func selectAnnotation_nilClearsSelection() async {
    let apps = [PlanningApplication.pendingReview]
    let (sut, _, _) = makeSUT(applications: apps)

    await sut.loadApplications()
    sut.selectApplication(PlanningApplication.pendingReview.id)
    sut.clearSelection()

    #expect(sut.selectedApplication == nil)
  }

  // MARK: - Empty State

  @Test func isEmpty_trueWhenNoAnnotationsAfterLoad() async {
    let (sut, _, _) = makeSUT(applications: [])

    await sut.loadApplications()

    #expect(sut.isEmpty)
  }

  @Test func isEmpty_falseWhenAnnotationsExist() async {
    let (sut, _, _) = makeSUT(applications: [.pendingReview])

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  @Test func isEmpty_falseWhileLoading() async {
    let (sut, _, _) = makeSUT()

    #expect(!sut.isEmpty)
  }

  @Test func isEmpty_falseWhenErrorOccurred() async {
    let (sut, spy, _) = makeSUT()
    // drainAllPages calls fetchApplicationsPage; set the paged error path.
    spy.fetchApplicationsPageError = DomainError.networkUnavailable

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  // MARK: - Zone-based loading

  @Test func loadApplications_fetchesBySelectedZone() async throws {
    let zone = try WatchZone(
      id: WatchZoneId("zone-1"),
      name: "Camden",
      centre: Coordinate(latitude: 51.539, longitude: -0.1426),
      radiusMetres: 1000,
      authorityId: 42
    )
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsByZone = ["zone-1": [.pendingReview]]
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([zone])

    let sut = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    await sut.loadApplications()

    #expect(spy.fetchApplicationsPageCalls.count == 1)
    #expect(spy.fetchApplicationsPageCalls[0].zone.id == zone.id)
    #expect(sut.annotations.count == 1)
  }

  @Test func loadApplications_returnsEmpty_whenNoZones() async {
    let spy = SpyPlanningApplicationRepository()
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([])

    let sut = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    await sut.loadApplications()

    #expect(spy.fetchApplicationsPageCalls.isEmpty)
    #expect(sut.annotations.isEmpty)
    #expect(sut.isEmpty)
  }

  // MARK: - Zone Selection

  private func makeSUTWithZones(
    zones: [WatchZone] = [.cambridge, .london],
    applications: [PlanningApplication] = [.pendingReview],
    persistedZoneId: String? = nil
  ) throws -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository, UserDefaults) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    if let persistedZoneId {
      defaults.set(persistedZoneId, forKey: "test.zone")
    }
    let vm = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )
    return (vm, appSpy, zoneSpy, defaults)
  }

  @Test func loadApplications_populatesZones() async throws {
    let (sut, _, _, _) = try makeSUTWithZones()

    await sut.loadApplications()

    #expect(sut.zones.count == 2)
    #expect(sut.zones[0].id == WatchZone.cambridge.id)
    #expect(sut.zones[1].id == WatchZone.london.id)
  }

  @Test func loadApplications_selectsFirstZoneByDefault() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones()

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsPageCalls.first?.zone.id == WatchZone.cambridge.id)
  }

  @Test func loadApplications_restoresPersistedZoneSelection() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones(persistedZoneId: "zone-002")

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsPageCalls.first?.zone.id == WatchZone.london.id)
    #expect(sut.centreLat == WatchZone.london.centre.latitude)
    #expect(sut.centreLon == WatchZone.london.centre.longitude)
  }

  @Test func loadApplications_fallsBackToFirstZone_whenPersistedZoneDeleted() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones(persistedZoneId: "zone-deleted")

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsPageCalls.first?.zone.id == WatchZone.cambridge.id)
  }

  @Test func selectZone_fetchesApplicationsAndUpdatesCentre() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    let initialCallCount = appSpy.fetchApplicationsPageCalls.count

    await sut.selectZone(.london)

    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsPageCalls.count == initialCallCount + 1)
    #expect(appSpy.fetchApplicationsPageCalls.last?.zone.id == WatchZone.london.id)
    #expect(sut.centreLat == WatchZone.london.centre.latitude)
    #expect(sut.centreLon == WatchZone.london.centre.longitude)
    #expect(sut.radiusMetres == WatchZone.london.radiusMetres)
  }

  @Test func selectZone_persistsSelectionToUserDefaults() async throws {
    let (sut, _, _, defaults) = try makeSUTWithZones()
    await sut.loadApplications()

    await sut.selectZone(.london)

    #expect(defaults.string(forKey: "test.zone") == "zone-002")
  }

  @Test func showZonePicker_trueWhenMultipleZones() async throws {
    let (sut, _, _, _) = try makeSUTWithZones()
    await sut.loadApplications()

    #expect(sut.showZonePicker)
  }

  @Test func showZonePicker_falseWhenSingleZone() async throws {
    let (sut, _, _, _) = try makeSUTWithZones(zones: [.cambridge])
    await sut.loadApplications()

    #expect(!sut.showZonePicker)
  }

  // MARK: - Eager page drain (GH#682 slice 5)

  @Test func loadApplications_drainsAllPages_followingCursorToExhaustion() async {
    let (sut, spy, _) = makeSUT()
    spy.pagedResponses = [
      ApplicationPage(applications: [.pendingReview], nextCursor: "cursor-1"),
      ApplicationPage(applications: [.permitted], nextCursor: "cursor-2"),
      ApplicationPage(applications: [.rejected], nextCursor: nil),
    ]

    await sut.loadApplications()

    // Every page's applications are merged into the published annotations.
    #expect(sut.annotations.count == 3)
    // The cursor threads through: first page sends none, then each prior page's
    // next-cursor drives the following request.
    #expect(spy.fetchApplicationsPageCalls.count == 3)
    #expect(spy.fetchApplicationsPageCalls[0].cursor == nil)
    #expect(spy.fetchApplicationsPageCalls[1].cursor == "cursor-1")
    #expect(spy.fetchApplicationsPageCalls[2].cursor == "cursor-2")
    // The map drains the cheapest (distance) plan, unfiltered.
    #expect(spy.fetchApplicationsPageCalls.allSatisfy { $0.sort == .distance })
    #expect(spy.fetchApplicationsPageCalls.allSatisfy { $0.filter == .all })
  }

  @Test func loadApplications_stopsDraining_onLastPageWithNoCursor() async {
    let (sut, spy, _) = makeSUT()
    spy.pagedResponses = [
      ApplicationPage(applications: [.pendingReview, .permitted], nextCursor: nil)
    ]

    await sut.loadApplications()

    #expect(spy.fetchApplicationsPageCalls.count == 1)
    #expect(sut.annotations.count == 2)
  }

  @Test func loadApplications_discardsPartialResults_whenAPageFails() async {
    let (sut, spy, _) = makeSUT()
    // Page 1 succeeds, page 2 throws: the drain aborts and the half-fetched set
    // is discarded rather than published as a partial map.
    spy.pagedResults = [
      .success(ApplicationPage(applications: [.pendingReview], nextCursor: "cursor-1")),
      .failure(DomainError.networkUnavailable),
    ]

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.annotations.isEmpty)
    #expect(spy.fetchApplicationsPageCalls.count == 2)
  }

  @Test func selectZone_drainsAllPages() async throws {
    let (sut, spy, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    spy.pagedResponses = [
      ApplicationPage(applications: [.pendingReview], nextCursor: "cursor-1"),
      ApplicationPage(applications: [.permitted], nextCursor: nil),
    ]

    await sut.selectZone(.london)

    let londonCalls = spy.fetchApplicationsPageCalls.filter { $0.zone.id == WatchZone.london.id }
    #expect(londonCalls.count == 2)
    #expect(sut.annotations.count == 2)
  }
}
