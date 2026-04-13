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
    let apps = [PlanningApplication.pendingReview, .approved, .refused, .withdrawn]
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
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.annotations.isEmpty)
  }

  // MARK: - Annotations

  @Test func annotations_haveCorrectStatus() async {
    let apps: [PlanningApplication] = [.pendingReview, .approved, .refused, .withdrawn]
    let (sut, _, _) = makeSUT(applications: apps)

    await sut.loadApplications()

    let pending = sut.annotations.first { $0.applicationId == PlanningApplicationId("APP-001") }
    let approved = sut.annotations.first { $0.applicationId == PlanningApplicationId("APP-002") }
    let refused = sut.annotations.first { $0.applicationId == PlanningApplicationId("APP-003") }
    let withdrawn = sut.annotations.first { $0.applicationId == PlanningApplicationId("APP-004") }

    #expect(pending?.status == .undecided)
    #expect(approved?.status == .approved)
    #expect(refused?.status == .refused)
    #expect(withdrawn?.status == .withdrawn)
  }

  @Test func annotations_onlyIncludeApplicationsWithLocations() async {
    let noLocation = PlanningApplication(
      id: PlanningApplicationId("APP-NO-LOC"),
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
    #expect(sut.annotations.first?.applicationId == PlanningApplicationId("APP-001"))
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
    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(sut.selectedApplication?.id == PlanningApplicationId("APP-001"))
  }

  @Test func selectAnnotation_nilClearsSelection() async {
    let apps = [PlanningApplication.pendingReview]
    let (sut, _, _) = makeSUT(applications: apps)

    await sut.loadApplications()
    sut.selectApplication(PlanningApplicationId("APP-001"))
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
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  // MARK: - Error Classification

  @Test func isNetworkError_trueForNetworkUnavailable() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.isNetworkError)
  }

  @Test func isNetworkError_falseForOtherErrors() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.unexpected("Server error"))

    await sut.loadApplications()

    #expect(!sut.isNetworkError)
  }

  @Test func isServerError_trueForServerError() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationsResult = .failure(
      DomainError.serverError(statusCode: 500, message: nil)
    )

    await sut.loadApplications()

    #expect(sut.isServerError)
    #expect(!sut.isNetworkError)
  }

  @Test func isServerError_falseForNetworkError() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isServerError)
    #expect(sut.isNetworkError)
  }

  @Test func isSessionExpired_trueForSessionExpired() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.sessionExpired)

    await sut.loadApplications()

    #expect(sut.isSessionExpired)
  }

  @Test func isSessionExpired_falseForOtherErrors() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isSessionExpired)
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

    #expect(spy.fetchApplicationsCalls.count == 1)
    #expect(spy.fetchApplicationsCalls[0].id == zone.id)
    #expect(sut.annotations.count == 1)
  }

  @Test func loadApplications_returnsEmpty_whenNoZones() async {
    let spy = SpyPlanningApplicationRepository()
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([])

    let sut = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    await sut.loadApplications()

    #expect(spy.fetchApplicationsCalls.isEmpty)
    #expect(sut.annotations.isEmpty)
    #expect(sut.isEmpty)
  }
}
