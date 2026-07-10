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

  @Test func init_seedsAnchorAndRadiusFromCoordinate() {
    let (sut, _) = makeSUT(coordinate: .cambridge, radiusMetres: 2000)

    #expect(sut.anchorCoordinate == .cambridge)
    #expect(sut.radiusMetres == 2000)
  }

  /// GH#912 Phase 4: the map no longer previews a free-tier-capped radius —
  /// the drawn circle and fetch boundary are always the zone's actual
  /// radius, even above the 2km free-tier cap (reachable via
  /// `DeviceLocalZoneEditorView`, whose bound is `DeviceLocalZone.maxRadiusMetres`
  /// = 5000m).
  @Test func init_seedsRadiusUnclamped_evenAboveFreeTierCap() {
    let (sut, _) = makeSUT(radiusMetres: 5000)

    #expect(sut.radiusMetres == 5000)
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

  // MARK: - View full details (GH#879 Phase 2)

  @Test func requestFullDetail_stashesSelectionAsPendingAndClearsSelection() {
    let (sut, _) = makeSUT()
    sut.selectApplication(.pendingReview)

    sut.requestFullDetail()

    #expect(sut.pendingDetailApplication == .pendingReview)
    #expect(sut.selectedApplication == nil)
  }

  @Test func requestFullDetail_isNoOp_whenNothingSelected() {
    let (sut, _) = makeSUT()

    sut.requestFullDetail()

    #expect(sut.pendingDetailApplication == nil)
  }

  @Test func presentPendingDetailIfNeeded_firesCallbackOnceWithPendingApplication() {
    let (sut, _) = makeSUT()
    sut.selectApplication(.pendingReview)
    sut.requestFullDetail()

    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }

    sut.presentPendingDetailIfNeeded()
    sut.presentPendingDetailIfNeeded()

    #expect(captured == [.pendingReview])
    #expect(sut.pendingDetailApplication == nil)
  }

  @Test func presentPendingDetailIfNeeded_noOpWhenNothingPending() {
    let (sut, _) = makeSUT()
    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }

    sut.presentPendingDetailIfNeeded()

    #expect(captured.isEmpty)
  }
}
