import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("SavedApplicationsView")
@MainActor
struct SavedApplicationsViewTests {

  // MARK: - Helpers

  private func makeViewModel(
    savedApplications: [SavedApplication] = []
  ) -> (SavedApplicationsViewModel, SpySavedApplicationRepository) {
    let spy = SpySavedApplicationRepository()
    spy.loadAllResult = .success(savedApplications)
    let vm = SavedApplicationsViewModel(repository: spy)
    return (vm, spy)
  }

  // MARK: - View Construction

  @Test("SavedApplicationsView can be constructed with empty state")
  func construction_emptyState_succeeds() {
    let (vm, _) = makeViewModel()

    let view = SavedApplicationsView(viewModel: vm)

    _ = view
  }

  @Test("SavedApplicationsView can be constructed with saved applications")
  func construction_withApplications_succeeds() {
    let (vm, _) = makeViewModel(savedApplications: [.rearExtension, .changeOfUse])

    let view = SavedApplicationsView(viewModel: vm)

    _ = view
  }
}
