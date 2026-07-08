import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 3: the anonymous Applications tab's list ViewModel — a
/// single nearest-first page over `AnonymousApplicationsRepository`, no
/// sort/filter chips (pre-resolved: v1 is nearest-first only).
@Suite("AnonymousApplicationListViewModel")
@MainActor
struct AnonymousApplicationListViewModelTests {
  private func makeSUT(
    coordinate: Coordinate = .cambridge,
    radiusMetres: Double = 2000
  ) -> (AnonymousApplicationListViewModel, SpyAnonymousApplicationsRepository) {
    let repository = SpyAnonymousApplicationsRepository()
    let sut = AnonymousApplicationListViewModel(
      repository: repository,
      coordinate: coordinate,
      radiusMetres: radiusMetres
    )
    return (sut, repository)
  }

  // MARK: - loadApplications

  @Test func loadApplications_fetchesNearestFirstAtSeededCoordinateAndRadius() async {
    let (sut, repository) = makeSUT(coordinate: .cambridge, radiusMetres: 1500)
    repository.fetchNearbyResult = .success([.pendingReview, .permitted])

    await sut.loadApplications()

    #expect(repository.fetchNearbyCalls.count == 1)
    let call = repository.fetchNearbyCalls[0]
    #expect(call.latitude == Coordinate.cambridge.latitude)
    #expect(call.longitude == Coordinate.cambridge.longitude)
    #expect(call.radiusMetres == 1500)
    #expect(call.limit == AnonymousApplicationListViewModel.defaultLimit)
    #expect(sut.applications == [.pendingReview, .permitted])
  }

  @Test func loadApplications_setsIsLoadingFalseAfterCompletion() async {
    let (sut, _) = makeSUT()

    #expect(!sut.isLoading)
    await sut.loadApplications()
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_failure_setsError() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.applications.isEmpty)
    #expect(sut.error == .networkUnavailable)
  }

  /// Pull-to-refresh calls the same `loadApplications()` entry point — a
  /// second successful fetch replaces the previously loaded rows.
  @Test func loadApplications_calledAgain_replacesPreviousApplications() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([.pendingReview])
    await sut.loadApplications()
    #expect(sut.applications == [.pendingReview])

    repository.fetchNearbyResult = .success([.permitted])
    await sut.loadApplications()

    #expect(sut.applications == [.permitted])
    #expect(repository.fetchNearbyCalls.count == 2)
  }

  // MARK: - isEmpty

  @Test func isEmpty_trueWhenNoApplicationsNoErrorNotLoading() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([])

    await sut.loadApplications()

    #expect(sut.isEmpty)
  }

  @Test func isEmpty_falseWhenApplicationsPresent() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .success([.pendingReview])

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  @Test func isEmpty_falseWhenErrorPresent() async {
    let (sut, repository) = makeSUT()
    repository.fetchNearbyResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  // MARK: - Row selection -> detail handoff (GH#879 Phase 2 established handoff)

  @Test func selectApplication_invokesOnShowApplicationDetail() {
    let (sut, _) = makeSUT()
    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }

    sut.selectApplication(.pendingReview)

    #expect(captured == [.pendingReview])
  }
}
