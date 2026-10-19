import Foundation
import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Construction/render smoke tests only: SwiftUI view trees aren't
/// introspectable here (no ViewInspector), as in `WatchZoneListViewTests`.
@Suite("SavedApplicationListView")
@MainActor
struct SavedApplicationListViewTests {

  private func makeViewModel() -> (
    SavedApplicationListViewModel, SpySavedApplicationRepository
  ) {
    let repo = SpySavedApplicationRepository()
    let vm = SavedApplicationListViewModel(savedApplicationRepository: repo)
    return (vm, repo)
  }

  @Test func body_renders_whenEmpty() {
    let (vm, _) = makeViewModel()
    let sut = SavedApplicationListView(viewModel: vm)
    _ = sut.body
  }

  @Test func body_renders_withApplications() async {
    let (vm, repo) = makeViewModel()
    repo.loadAllResult = .success([.fixture(uid: "APP-A")])
    await vm.loadAll()

    let sut = SavedApplicationListView(viewModel: vm)
    _ = sut.body
  }
}
