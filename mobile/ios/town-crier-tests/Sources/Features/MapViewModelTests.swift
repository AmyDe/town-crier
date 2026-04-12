import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("MapViewModel")
@MainActor
struct MapViewModelTests {
  private func makeSUT(
    applications: [PlanningApplication] = [],
    watchZone: WatchZone = .cambridge
  ) -> (MapViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(applications)
    let vm = MapViewModel(repository: spy, watchZone: watchZone)
    return (vm, spy)
  }

  private func makeSUTWithAuthorities(
    authorities: [LocalAuthority] = [.cambridge],
    applicationsByAuthority: [String: [PlanningApplication]] = [:],
    watchZone: WatchZone = .cambridge
  ) -> (MapViewModel, SpyApplicationAuthorityRepository, SpyPlanningApplicationRepository) {
    let authoritySpy = SpyApplicationAuthorityRepository()
    authoritySpy.fetchAuthoritiesResult = .success(
      ApplicationAuthorityResult(authorities: authorities, count: authorities.count)
    )
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsByAuthority = applicationsByAuthority
    let vm = MapViewModel(
      authorityRepository: authoritySpy,
      applicationRepository: appSpy,
      watchZone: watchZone
    )
    return (vm, authoritySpy, appSpy)
  }

  // MARK: - Loading

  @Test func loadApplications_populatesAnnotations() async {
    let apps = [PlanningApplication.pendingReview, .approved, .refused, .withdrawn]
    let (sut, _) = makeSUT(applications: apps)

    await sut.loadApplications()

    #expect(sut.annotations.count == 4)
  }

  @Test func loadApplications_setsIsLoadingDuringFetch() async {
    let (sut, _) = makeSUT()

    #expect(!sut.isLoading)
    await sut.loadApplications()
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_setsErrorOnFailure() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.annotations.isEmpty)
  }

  // MARK: - Annotations

  @Test func annotations_haveCorrectStatus() async {
    let apps: [PlanningApplication] = [.pendingReview, .approved, .refused, .withdrawn]
    let (sut, _) = makeSUT(applications: apps)

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
    let (sut, _) = makeSUT(applications: [.pendingReview, noLocation])

    await sut.loadApplications()

    #expect(sut.annotations.count == 1)
    #expect(sut.annotations.first?.applicationId == PlanningApplicationId("APP-001"))
  }

  // MARK: - Watch zone

  @Test func watchZoneCentre_matchesProvidedZone() throws {
    let zone = try WatchZone(
      postcode: Postcode("SW1A 1AA"),
      centre: Coordinate(latitude: 51.5, longitude: -0.1),
      radiusMetres: 3000
    )
    let (sut, _) = makeSUT(watchZone: zone)

    #expect(sut.centreLat == 51.5)
    #expect(sut.centreLon == -0.1)
    #expect(sut.radiusMetres == 3000)
  }

  // MARK: - Selection

  @Test func selectAnnotation_setsSelectedApplication() async {
    let apps = [PlanningApplication.pendingReview]
    let (sut, _) = makeSUT(applications: apps)

    await sut.loadApplications()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(sut.selectedApplication?.id == PlanningApplicationId("APP-001"))
  }

  @Test func selectAnnotation_nilClearsSelection() async {
    let apps = [PlanningApplication.pendingReview]
    let (sut, _) = makeSUT(applications: apps)

    await sut.loadApplications()
    sut.selectApplication(PlanningApplicationId("APP-001"))
    sut.clearSelection()

    #expect(sut.selectedApplication == nil)
  }

  // MARK: - Empty State

  @Test func isEmpty_trueWhenNoAnnotationsAfterLoad() async {
    let (sut, _) = makeSUT(applications: [])

    await sut.loadApplications()

    #expect(sut.isEmpty)
  }

  @Test func isEmpty_falseWhenAnnotationsExist() async {
    let (sut, _) = makeSUT(applications: [.pendingReview])

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  @Test func isEmpty_falseWhileLoading() async {
    let (sut, _) = makeSUT()

    #expect(!sut.isEmpty)
  }

  @Test func isEmpty_falseWhenErrorOccurred() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  // MARK: - Error Classification

  @Test func isNetworkError_trueForNetworkUnavailable() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.isNetworkError)
  }

  @Test func isNetworkError_falseForOtherErrors() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.unexpected("Server error"))

    await sut.loadApplications()

    #expect(!sut.isNetworkError)
  }

  @Test func isSessionExpired_trueForSessionExpired() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.sessionExpired)

    await sut.loadApplications()

    #expect(sut.isSessionExpired)
  }

  @Test func isSessionExpired_falseForOtherErrors() async {
    let (sut, spy) = makeSUT()
    spy.fetchApplicationsResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isSessionExpired)
  }

  // MARK: - Authority-based loading

  @Test func loadApplications_fetchesAuthoritiesThenApplications() async {
    let cambridge = LocalAuthority(code: "CAM", name: "Cambridge")
    let apps = [PlanningApplication.pendingReview]
    let (sut, authSpy, appSpy) = makeSUTWithAuthorities(
      authorities: [cambridge],
      applicationsByAuthority: ["CAM": apps]
    )

    await sut.loadApplications()

    #expect(authSpy.fetchAuthoritiesCallCount == 1)
    #expect(appSpy.fetchApplicationsCalls.map(\.code) == ["CAM"])
    #expect(sut.annotations.count == 1)
  }

  @Test func loadApplications_combinesApplicationsFromMultipleAuthorities() async {
    let cambridge = LocalAuthority(code: "CAM", name: "Cambridge")
    let oxford = LocalAuthority(code: "OXF", name: "Oxford")
    let (sut, _, _) = makeSUTWithAuthorities(
      authorities: [cambridge, oxford],
      applicationsByAuthority: [
        "CAM": [.pendingReview],
        "OXF": [.approved],
      ]
    )

    await sut.loadApplications()

    #expect(sut.annotations.count == 2)
  }

  @Test func loadApplications_authorityFetchFailure_setsError() async {
    let (sut, authSpy, _) = makeSUTWithAuthorities()
    authSpy.fetchAuthoritiesResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.annotations.isEmpty)
  }

  @Test func loadApplications_noAuthorities_resultsInEmptyAnnotations() async {
    let (sut, _, _) = makeSUTWithAuthorities(authorities: [])

    await sut.loadApplications()

    #expect(sut.annotations.isEmpty)
    #expect(sut.isEmpty)
  }

  @Test func loadApplications_partialAuthorityFailure_showsPartialResults() async {
    let cambridge = LocalAuthority(code: "CAM", name: "Cambridge")
    let oxford = LocalAuthority(code: "OXF", name: "Oxford")
    let (sut, _, appSpy) = makeSUTWithAuthorities(
      authorities: [cambridge, oxford],
      applicationsByAuthority: [
        "CAM": [.pendingReview]
      ]
    )
    appSpy.fetchApplicationsFailureAuthorities = ["OXF"]

    await sut.loadApplications()

    #expect(sut.annotations.count == 1)
  }
}
