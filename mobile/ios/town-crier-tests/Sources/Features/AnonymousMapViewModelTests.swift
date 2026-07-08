import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AnonymousMapViewModel")
@MainActor
struct AnonymousMapViewModelTests {
  private func makeSUT(
    coordinate: Coordinate = .cambridge,
    radiusMetres: Double = 2000,
    debounceNanoseconds: UInt64 = 5_000_000
  ) -> (AnonymousMapViewModel, SpyAnonymousApplicationsRepository) {
    let repository = SpyAnonymousApplicationsRepository()
    let sut = AnonymousMapViewModel(
      repository: repository,
      coordinate: coordinate,
      radiusMetres: radiusMetres,
      debounceNanoseconds: debounceNanoseconds
    )
    return (sut, repository)
  }

  // MARK: - Initial state

  @Test func init_seedsCentreAndRadiusFromCoordinate() {
    let (sut, _) = makeSUT(coordinate: .cambridge, radiusMetres: 2000)

    #expect(sut.centreLat == Coordinate.cambridge.latitude)
    #expect(sut.centreLon == Coordinate.cambridge.longitude)
    #expect(sut.radiusMetres == 2000)
  }

  // MARK: - loadInitial

  @Test func loadInitial_fetchesAtSeededCoordinateAndRadius() async {
    let (sut, repository) = makeSUT(coordinate: .cambridge, radiusMetres: 2000)
    repository.fetchNearbyResult = .success([.pendingReview])

    await sut.loadInitial()

    #expect(repository.fetchNearbyCalls.count == 1)
    let call = repository.fetchNearbyCalls[0]
    #expect(call.latitude == Coordinate.cambridge.latitude)
    #expect(call.longitude == Coordinate.cambridge.longitude)
    #expect(call.radiusMetres == 2000)
    #expect(sut.applications == [.pendingReview])
  }

  @Test func loadInitial_setsIsLoadingFalseAfterCompletion() async {
    let (sut, _) = makeSUT()

    #expect(!sut.isLoading)
    await sut.loadInitial()
    #expect(!sut.isLoading)
  }

  @Test func loadInitial_failure_setsErrorWhenApplicationsEmpty() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .failure(DomainError.networkUnavailable)

    await sut.loadInitial()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.applications.isEmpty)
  }

  // MARK: - regionDidChange

  @Test func regionDidChange_clampsRadiusToServerMax() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([])

    sut.regionDidChange(centreLat: 51.5, centreLon: -0.1, radiusMetres: 9000)
    await sut.waitForPendingRegionChangeRefetch()

    #expect(sut.radiusMetres == 5000)
    #expect(repository.fetchNearbyCalls.last?.radiusMetres == 5000)
  }

  @Test func regionDidChange_clampsRadiusToServerMin() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([])

    sut.regionDidChange(centreLat: 51.5, centreLon: -0.1, radiusMetres: 10)
    await sut.waitForPendingRegionChangeRefetch()

    #expect(sut.radiusMetres == 100)
    #expect(repository.fetchNearbyCalls.last?.radiusMetres == 100)
  }

  @Test func regionDidChange_updatesCentre() async {
    let (sut, _) = makeSUT()

    sut.regionDidChange(centreLat: 51.5, centreLon: -0.1, radiusMetres: 2000)
    await sut.waitForPendingRegionChangeRefetch()

    #expect(sut.centreLat == 51.5)
    #expect(sut.centreLon == -0.1)
  }

  @Test func regionDidChange_debounces_rapidCallsIssueOneRefetch() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([])

    sut.regionDidChange(centreLat: 51.0, centreLon: -0.1, radiusMetres: 2000)
    sut.regionDidChange(centreLat: 52.0, centreLon: -0.2, radiusMetres: 3000)
    await sut.waitForPendingRegionChangeRefetch()

    #expect(repository.fetchNearbyCalls.count == 1)
    #expect(repository.fetchNearbyCalls[0].latitude == 52.0)
    #expect(repository.fetchNearbyCalls[0].radiusMetres == 3000)
  }

  @Test func regionDidChange_populatesApplicationsAfterDebounce() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([.permitted])

    sut.regionDidChange(centreLat: 51.5, centreLon: -0.1, radiusMetres: 2000)
    await sut.waitForPendingRegionChangeRefetch()

    #expect(sut.applications == [.permitted])
  }

  // MARK: - Selection

  @Test func selectApplication_setsSelectedApplication() {
    let (sut, _) = makeSUT()

    sut.selectApplication(.pendingReview)

    #expect(sut.selectedApplication == .pendingReview)
  }

  @Test func clearSelection_clearsSelectedApplication() {
    let (sut, _) = makeSUT()
    sut.selectApplication(.pendingReview)

    sut.clearSelection()

    #expect(sut.selectedApplication == nil)
  }

  // MARK: - Sign-up handoff

  @Test func requestSignUp_invokesCallback() {
    let (sut, _) = makeSUT()
    var invoked = false
    sut.onRequestSignUp = { invoked = true }

    sut.requestSignUp()

    #expect(invoked)
  }

  // MARK: - Live radius picker (GH#868 Phase 3 refinement)

  @Test func selectedRadiusMetres_seedsFromInitialRadius() {
    let (sut, _) = makeSUT(radiusMetres: 1500)

    #expect(sut.selectedRadiusMetres == 1500)
  }

  @Test func selectedRadiusMetres_clampsToFreeTierMaxWhenInitialRadiusIsLarger() {
    // The fetch radius (server clamp: [100, 5000]) can exceed the free-tier
    // cap the live picker is bounded to — the seeded picker value must never
    // preview a zone bigger than a free account can actually have.
    let (sut, _) = makeSUT(radiusMetres: 5000)

    #expect(sut.selectedRadiusMetres == 2000)
  }

  @Test func maxSelectedRadiusMetres_matchesFreeTierCap() {
    #expect(AnonymousMapViewModel.maxSelectedRadiusMetres == 2000)
  }

  @Test func updateSelectedRadius_updatesValue() {
    let (sut, _) = makeSUT()

    sut.updateSelectedRadius(750)

    #expect(sut.selectedRadiusMetres == 750)
  }

  @Test func updateSelectedRadius_clampsAboveFreeTierMax() {
    let (sut, _) = makeSUT()

    sut.updateSelectedRadius(3000)

    #expect(sut.selectedRadiusMetres == 2000)
  }

  @Test func updateSelectedRadius_clampsBelowMinimum() {
    let (sut, _) = makeSUT()

    sut.updateSelectedRadius(10)

    #expect(sut.selectedRadiusMetres == 100)
  }

  @Test func updateSelectedRadius_invokesOnRadiusChangedWithClampedValue() {
    let (sut, _) = makeSUT()
    var received: Double?
    sut.onRadiusChanged = { received = $0 }

    sut.updateSelectedRadius(9000)

    #expect(received == 2000)
  }

  @Test func anchorCoordinate_staysFixedAcrossRegionChanges() async {
    let (sut, repository) = makeSUT(coordinate: .cambridge)
    repository.fetchNearbyResult = .success([])

    sut.regionDidChange(centreLat: 10, centreLon: 10, radiusMetres: 2000)
    await sut.waitForPendingRegionChangeRefetch()

    #expect(sut.anchorCoordinate == .cambridge)
    // The viewport-following centre moved, but the anchor the radius circle
    // is drawn around did not — the two are deliberately decoupled.
    #expect(sut.centreLat == 10)
  }

  // MARK: - Stacked (same-address) cluster disambiguation (GH#877)

  @Test func selectStack_publishesStackedApplicationsWithAllMembersInOrder() {
    let (sut, _) = makeSUT()
    let members = [PlanningApplication.pendingReview, .permitted, .rejected]

    sut.selectStack(members)

    #expect(sut.stackedApplications?.applications == members)
  }

  @Test func selectStack_makesNoRepositoryCall() {
    // Unlike the authenticated map's `MapViewModel.selectStack(_:)`, the
    // anonymous map already holds full `PlanningApplication` objects from the
    // `near-point` fetch — no point read is needed.
    let (sut, repository) = makeSUT()

    sut.selectStack([.pendingReview, .permitted])

    #expect(repository.fetchNearbyCalls.isEmpty)
  }

  @Test func selectFromStack_clearsStackedApplicationsWithoutImmediatelySelecting() {
    let (sut, _) = makeSUT()
    sut.selectStack([.pendingReview, .permitted])

    sut.selectFromStack(.permitted)

    // The list dismisses first — the summary must NOT be up yet (no two
    // sheets at once).
    #expect(sut.stackedApplications == nil)
    #expect(sut.selectedApplication == nil)
  }

  @Test func presentPendingSummaryIfNeeded_presentsTheChosenApplicationAfterListDismisses() {
    let (sut, _) = makeSUT()
    sut.selectStack([.pendingReview, .permitted])
    sut.selectFromStack(.permitted)

    sut.presentPendingSummaryIfNeeded()

    #expect(sut.selectedApplication == .permitted)
  }

  @Test func presentPendingSummaryIfNeeded_noOpWhenNothingPending() {
    let (sut, _) = makeSUT()

    sut.presentPendingSummaryIfNeeded()

    #expect(sut.selectedApplication == nil)
  }

  @Test func clearStack_clearsStackedApplications() {
    let (sut, _) = makeSUT()
    sut.selectStack([.pendingReview, .permitted])

    sut.clearStack()

    #expect(sut.stackedApplications == nil)
  }
}
