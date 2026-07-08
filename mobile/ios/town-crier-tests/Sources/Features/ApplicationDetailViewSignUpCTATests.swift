import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 2: the anonymous detail screen replaces the Save affordance
/// with a sign-up CTA.
@Suite("ApplicationDetailView — sign-up CTA")
@MainActor
struct ApplicationDetailViewSignUpCTATests {

  @Test("view renders when the view model is in anonymous mode")
  func construction_inAnonymousMode_rendersWithoutCrashing() {
    let vm = ApplicationDetailViewModel(
      application: .pendingReview,
      anonymousApplicationDetailRepository: SpyAnonymousApplicationDetailRepository()
    )
    let view = ApplicationDetailView(viewModel: vm)

    _ = view.body
  }

  @Test("anonymous view model hides Save and shows the sign-up CTA")
  func anonymousViewModel_hidesSaveAndShowsSignUpCTA() {
    let vm = ApplicationDetailViewModel(
      application: .pendingReview,
      anonymousApplicationDetailRepository: SpyAnonymousApplicationDetailRepository()
    )

    #expect(!vm.canSave)
    #expect(vm.showsSignUpCTA)
  }

  @Test("authed view model with a save repository never shows the sign-up CTA")
  func authedViewModel_neverShowsSignUpCTA() {
    let vm = ApplicationDetailViewModel(
      application: .pendingReview,
      savedApplicationRepository: SpySavedApplicationRepository()
    )

    #expect(vm.canSave)
    #expect(!vm.showsSignUpCTA)
  }
}
