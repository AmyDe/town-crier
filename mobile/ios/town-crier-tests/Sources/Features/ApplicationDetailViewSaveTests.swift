import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationDetailView — save button")
@MainActor
struct ApplicationDetailViewSaveTests {

  @Test("view can be constructed with save-capable ViewModel")
  func construction_withSaveRepository_succeeds() {
    let spy = SpySavedApplicationRepository()
    let vm = ApplicationDetailViewModel(
      application: .pendingReview,
      savedApplicationRepository: spy,
      isSaved: false
    )

    let view = ApplicationDetailView(viewModel: vm)

    _ = view
  }

  @Test("view can be constructed with saved state true")
  func construction_withSavedTrue_succeeds() {
    let spy = SpySavedApplicationRepository()
    let vm = ApplicationDetailViewModel(
      application: .pendingReview,
      savedApplicationRepository: spy,
      isSaved: true
    )

    let view = ApplicationDetailView(viewModel: vm)

    _ = view
  }

  @Test("view can be constructed without save repository")
  func construction_withoutRepository_succeeds() {
    let vm = ApplicationDetailViewModel(application: .pendingReview)

    let view = ApplicationDetailView(viewModel: vm)

    _ = view
  }
}
