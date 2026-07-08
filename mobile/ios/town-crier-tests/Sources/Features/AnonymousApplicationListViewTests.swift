import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AnonymousApplicationListView")
@MainActor
struct AnonymousApplicationListViewTests {
  private func makeViewModel(
    coordinate: Coordinate = .cambridge, radiusMetres: Double = 2000
  ) -> (AnonymousApplicationListViewModel, SpyAnonymousApplicationsRepository) {
    let repository = SpyAnonymousApplicationsRepository()
    let viewModel = AnonymousApplicationListViewModel(
      repository: repository, coordinate: coordinate, radiusMetres: radiusMetres)
    return (viewModel, repository)
  }

  @Test func body_renders_whenEmpty() {
    let (viewModel, _) = makeViewModel()
    let sut = AnonymousApplicationListView(viewModel: viewModel)

    _ = sut.body
  }

  @Test func body_renders_withApplicationsLoaded() async {
    let (viewModel, repository) = makeViewModel()
    repository.fetchNearbyResult = .success([.pendingReview, .permitted])
    await viewModel.loadApplications()
    let sut = AnonymousApplicationListView(viewModel: viewModel)

    _ = sut.body
  }

  @Test func body_renders_withErrorState() async {
    let (viewModel, repository) = makeViewModel()
    repository.fetchNearbyResult = .failure(DomainError.networkUnavailable)
    await viewModel.loadApplications()
    let sut = AnonymousApplicationListView(viewModel: viewModel)

    _ = sut.body
  }

  @Test func rowTap_invokesOnShowApplicationDetail() {
    let (viewModel, _) = makeViewModel()
    var captured: [PlanningApplication] = []
    viewModel.onShowApplicationDetail = { captured.append($0) }

    // Mirrors the row's `.onTapGesture` wiring — the tap gesture itself is
    // not exercisable without UI-level automation, so this asserts the same
    // ViewModel call the gesture invokes (mirrors `ApplicationListView`'s
    // reliance on `selectApplication` at the ViewModel level).
    viewModel.selectApplication(.pendingReview)

    #expect(captured == [.pendingReview])
  }
}
