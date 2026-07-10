import Foundation
import SwiftUI
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// View-level regression coverage for the Saved masthead fix (GH#912 Phase
/// 1): the tab previously showed only the system nav-bar title in the system
/// font, with no `MastheadView` row wired in at all. `WatchZoneListViewTests`
/// establishes the pattern this suite mirrors — SwiftUI view trees aren't
/// introspectable in this codebase (no ViewInspector), so these are
/// construction/render smoke tests; the meaningful regression guard is
/// architectural: `SavedApplicationListView.body` now composes a
/// `mastheadRow` as the first row of the `List`, alongside
/// `.mastheadNavigationBar()`.
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
