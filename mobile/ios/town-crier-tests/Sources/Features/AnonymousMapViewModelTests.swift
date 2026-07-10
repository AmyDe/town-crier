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
  ) -> (AnonymousMapViewModel, SpyAnonymousApplicationsRepository, SpyAnonymousApplicationDetailRepository) {
    let repository = SpyAnonymousApplicationsRepository()
    let detailRepository = SpyAnonymousApplicationDetailRepository()
    let sut = AnonymousMapViewModel(
      repository: repository,
      detailRepository: detailRepository,
      coordinate: coordinate,
      radiusMetres: radiusMetres,
      debounceNanoseconds: debounceNanoseconds
    )
    return (sut, repository, detailRepository)
  }

  // MARK: - Initial state

  @Test func init_seedsAnchorAndRadiusFromCoordinate() {
    let (sut, _, _) = makeSUT(coordinate: .cambridge, radiusMetres: 2000)

    #expect(sut.anchorCoordinate == .cambridge)
    #expect(sut.radiusMetres == 2000)
  }

  /// GH#912 Phase 4: the map no longer previews a free-tier-capped radius —
  /// the drawn circle and fetch boundary are always the zone's actual
  /// radius, even above the 2km free-tier cap (reachable via
  /// `DeviceLocalZoneEditorView`, whose bound is `DeviceLocalZone.maxRadiusMetres`
  /// = 5000m).
  @Test func init_seedsRadiusUnclamped_evenAboveFreeTierCap() {
    let (sut, _, _) = makeSUT(radiusMetres: 5000)

    #expect(sut.radiusMetres == 5000)
  }

  // MARK: - loadInitial / loadClusters (GH#924 Phase 2)

  @Test func loadInitial_fetchesClustersForTheAnchorDerivedViewport() async {
    let (sut, repository, _) = makeSUT(coordinate: .cambridge, radiusMetres: 2000)
    repository.fetchClustersResult = .success([.bubble(count: 5)])
    let (expectedViewport, expectedZoom) = MapViewModel.initialViewport(
      centre: .cambridge, radiusMetres: 2000)

    await sut.loadInitial()

    #expect(repository.fetchClustersCalls.count == 1)
    let call = repository.fetchClustersCalls[0]
    #expect(call.latitude == Coordinate.cambridge.latitude)
    #expect(call.longitude == Coordinate.cambridge.longitude)
    #expect(call.radiusMetres == 2000)
    #expect(call.viewport == expectedViewport)
    #expect(call.zoom == expectedZoom)
    #expect(sut.clusters == [.bubble(count: 5)])
  }

  @Test func loadInitial_setsIsLoadingFalseAfterCompletion() async {
    let (sut, _, _) = makeSUT()

    #expect(!sut.isLoading)
    await sut.loadInitial()
    #expect(!sut.isLoading)
  }

  @Test func loadInitial_failure_setsErrorWhenClustersEmpty() async {
    let (sut, repository, _) = makeSUT()
    repository.fetchClustersResult = .failure(DomainError.networkUnavailable)

    await sut.loadInitial()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.clusters.isEmpty)
  }

  @Test func loadClusters_fetchesAndPublishesTheGivenViewport() async {
    let (sut, repository, _) = makeSUT()
    let viewport = MapViewport.test
    repository.fetchClustersResult = .success([.bubble(count: 9)])

    await sut.loadClusters(viewport: viewport, zoom: 14)

    let call = repository.fetchClustersCalls.last
    #expect(call?.viewport == viewport)
    #expect(call?.zoom == 14)
    // The anchor/radius still drive the fetch, independent of the viewport
    // the caller supplies (mirrors `MapViewModel.loadClusters`).
    #expect(call?.latitude == sut.anchorCoordinate.latitude)
    #expect(call?.radiusMetres == sut.radiusMetres)
    #expect(sut.clusters == [.bubble(count: 9)])
  }

  @Test func loadClusters_transientFailure_keepsLastGoodClustersAndNoError() async {
    let (sut, repository, _) = makeSUT()
    repository.fetchClustersResult = .success([.bubble(count: 2)])
    await sut.loadInitial()
    repository.fetchClustersResult = .failure(DomainError.networkUnavailable)

    await sut.loadClusters(viewport: .test, zoom: 12)

    #expect(sut.clusters == [.bubble(count: 2)])
    #expect(sut.error == nil)
  }

  // MARK: - Selection

  @Test func selectApplication_setsSelectedApplication() {
    let (sut, _, _) = makeSUT()

    sut.selectApplication(.pendingReview)

    #expect(sut.selectedApplication == .pendingReview)
  }

  @Test func clearSelection_clearsSelectedApplication() {
    let (sut, _, _) = makeSUT()
    sut.selectApplication(.pendingReview)

    sut.clearSelection()

    #expect(sut.selectedApplication == nil)
  }

  // MARK: - Sign-up handoff

  @Test func requestSignUp_invokesCallback() {
    let (sut, _, _) = makeSUT()
    var invoked = false
    sut.onRequestSignUp = { invoked = true }

    sut.requestSignUp()

    #expect(invoked)
  }

  // MARK: - Single-member cluster tap (by-slug point-read, GH#924)

  @Test func selectCluster_singleMember_pointReadsBySlugAndPresentsSummary() async {
    let (sut, _, detailRepository) = makeSUT()
    let member = AnonymousClusterMember.kingstonOne
    detailRepository.fetchApplicationBySlugResult = .success(.permitted)

    await sut.selectCluster(.single(member: member))

    #expect(
      detailRepository.fetchApplicationBySlugCalls == [
        SpyAnonymousApplicationDetailRepository.RecordedBySlugRequest(
          authoritySlug: member.authoritySlug, ref: member.value)
      ])
    #expect(sut.selectedApplication == .permitted)
  }

  @Test func selectCluster_bubble_doesNothing() async {
    let (sut, _, detailRepository) = makeSUT()

    await sut.selectCluster(.bubble(count: 5))

    #expect(detailRepository.fetchApplicationBySlugCalls.isEmpty)
    #expect(sut.selectedApplication == nil)
  }

  @Test func selectCluster_pointReadFailure_leavesMapUntouched() async {
    let (sut, _, detailRepository) = makeSUT()
    detailRepository.fetchApplicationBySlugResult = .failure(DomainError.networkUnavailable)

    await sut.selectCluster(.single(member: .kingstonOne))

    #expect(sut.selectedApplication == nil)
    #expect(sut.error == nil)
  }

  @Test func selectCluster_missingSlug_ignoresTapSilently() async {
    let (sut, _, detailRepository) = makeSUT()

    await sut.selectCluster(.single(member: .missingSlug))

    #expect(detailRepository.fetchApplicationBySlugCalls.isEmpty)
    #expect(sut.selectedApplication == nil)
  }

  // MARK: - Stacked cluster tap (all-or-nothing point-reads, GH#924)

  @Test func selectStack_fetchesEachMemberBySlugAndPublishesOrderedList() async {
    let (sut, _, detailRepository) = makeSUT()
    let members = [
      AnonymousClusterMember.kingstonOne, .kingstonTwo, .kingstonThree,
    ]
    detailRepository.fetchApplicationBySlugResultsByRef = [
      AnonymousClusterMember.kingstonOne.value: .success(.pendingReview),
      AnonymousClusterMember.kingstonTwo.value: .success(.permitted),
      AnonymousClusterMember.kingstonThree.value: .success(.rejected),
    ]

    await sut.selectStack(.stacked(members: members))

    // Publishes one application per member, in the cluster's member order (a
    // TaskGroup completes out of order, so the result must be reindexed).
    #expect(sut.stackedApplications?.applications == [.pendingReview, .permitted, .rejected])
    #expect(detailRepository.fetchApplicationBySlugCalls.count == 3)
    #expect(sut.selectedApplication == nil)
  }

  @Test func selectStack_leavesMapUntouched_whenAMemberReadFails() async {
    let (sut, _, detailRepository) = makeSUT()
    let members = [AnonymousClusterMember.kingstonOne, .kingstonTwo]
    detailRepository.fetchApplicationBySlugResultsByRef = [
      AnonymousClusterMember.kingstonOne.value: .success(.pendingReview),
      AnonymousClusterMember.kingstonTwo.value: .failure(DomainError.networkUnavailable),
    ]

    await sut.selectStack(.stacked(members: members))

    // All-or-nothing: one failed member publishes no list and never blanks
    // the map with an error — the user can tap again.
    #expect(sut.stackedApplications == nil)
    #expect(sut.error == nil)
  }

  @Test func selectStack_missingSlugOnAnyMember_ignoresTapSilently() async {
    let (sut, _, detailRepository) = makeSUT()
    let members = [AnonymousClusterMember.kingstonOne, .missingSlug]

    await sut.selectStack(.stacked(members: members))

    #expect(detailRepository.fetchApplicationBySlugCalls.isEmpty)
    #expect(sut.stackedApplications == nil)
  }

  @Test func selectStack_nonStackedCluster_doesNothing() async {
    let (sut, _, detailRepository) = makeSUT()

    await sut.selectStack(.bubble(count: 42))

    #expect(sut.stackedApplications == nil)
    #expect(detailRepository.fetchApplicationBySlugCalls.isEmpty)
  }

  @Test func selectFromStack_clearsStackedApplicationsWithoutImmediatelySelecting() async {
    let (sut, _, detailRepository) = makeSUT()
    let members = [AnonymousClusterMember.kingstonOne, .kingstonTwo]
    detailRepository.fetchApplicationBySlugResultsByRef = [
      AnonymousClusterMember.kingstonOne.value: .success(.pendingReview),
      AnonymousClusterMember.kingstonTwo.value: .success(.permitted),
    ]
    await sut.selectStack(.stacked(members: members))

    sut.selectFromStack(.permitted)

    // The list dismisses first — the summary must NOT be up yet (no two
    // sheets at once).
    #expect(sut.stackedApplications == nil)
    #expect(sut.selectedApplication == nil)
  }

  @Test func presentPendingSummaryIfNeeded_presentsTheChosenApplicationAfterListDismisses() async {
    let (sut, _, detailRepository) = makeSUT()
    let members = [AnonymousClusterMember.kingstonOne, .kingstonTwo]
    detailRepository.fetchApplicationBySlugResultsByRef = [
      AnonymousClusterMember.kingstonOne.value: .success(.pendingReview),
      AnonymousClusterMember.kingstonTwo.value: .success(.permitted),
    ]
    await sut.selectStack(.stacked(members: members))
    sut.selectFromStack(.permitted)

    sut.presentPendingSummaryIfNeeded()

    #expect(sut.selectedApplication == .permitted)
  }

  @Test func presentPendingSummaryIfNeeded_noOpWhenNothingPending() {
    let (sut, _, _) = makeSUT()

    sut.presentPendingSummaryIfNeeded()

    #expect(sut.selectedApplication == nil)
  }

  @Test func clearStack_clearsStackedApplications() async {
    let (sut, _, detailRepository) = makeSUT()
    let members = [AnonymousClusterMember.kingstonOne, .kingstonTwo]
    detailRepository.fetchApplicationBySlugResultsByRef = [
      AnonymousClusterMember.kingstonOne.value: .success(.pendingReview),
      AnonymousClusterMember.kingstonTwo.value: .success(.permitted),
    ]
    await sut.selectStack(.stacked(members: members))

    sut.clearStack()

    #expect(sut.stackedApplications == nil)
  }

  // MARK: - View full details (GH#879 Phase 2)

  @Test func requestFullDetail_stashesSelectionAsPendingAndClearsSelection() {
    let (sut, _, _) = makeSUT()
    sut.selectApplication(.pendingReview)

    sut.requestFullDetail()

    #expect(sut.pendingDetailApplication == .pendingReview)
    #expect(sut.selectedApplication == nil)
  }

  @Test func requestFullDetail_isNoOp_whenNothingSelected() {
    let (sut, _, _) = makeSUT()

    sut.requestFullDetail()

    #expect(sut.pendingDetailApplication == nil)
  }

  @Test func presentPendingDetailIfNeeded_firesCallbackOnceWithPendingApplication() {
    let (sut, _, _) = makeSUT()
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
    let (sut, _, _) = makeSUT()
    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }

    sut.presentPendingDetailIfNeeded()

    #expect(captured.isEmpty)
  }
}
