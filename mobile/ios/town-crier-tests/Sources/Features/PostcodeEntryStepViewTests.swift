import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
@Suite("PostcodeEntryStepView")
struct PostcodeEntryStepViewTests {

  // The step lets the user watch ANY UK postcode, not just their home address
  // (tc-rae0). Copy is exposed as constants so the wording lives in one place and
  // can be asserted directly.

  private func makeViewModel(tier: SubscriptionTier = .free) -> OnboardingViewModel {
    OnboardingViewModel(
      geocoder: SpyPostcodeGeocoder(),
      watchZoneRepository: SpyWatchZoneRepository(),
      onboardingRepository: SpyOnboardingRepository(),
      notificationService: SpyNotificationService(),
      subscriptionTier: tier
    )
  }

  // MARK: - Title

  @Test func title_doesNotImplyHomeOnly() {
    // The old title ("Where do you live?") implied the postcode had to be home.
    #expect(PostcodeEntryStepView.Copy.title != "Where do you live?")
    #expect(!PostcodeEntryStepView.Copy.title.lowercased().contains("live"))
  }

  @Test func title_exactWording() {
    #expect(PostcodeEntryStepView.Copy.title == "Pick a postcode to watch")
  }

  // MARK: - Helper

  @Test func helper_statesAnyPostcodeWorks() {
    #expect(PostcodeEntryStepView.Copy.helper.contains("Any UK postcode"))
  }

  @Test func helper_givesConcreteExamples() {
    let helper = PostcodeEntryStepView.Copy.helper
    #expect(helper.contains("home"))
    #expect(helper.contains("work"))
    #expect(helper.contains("relative's street"))
    #expect(helper.contains("keeping an eye on"))
  }

  @Test func helper_doesNotImplyHomeOnly() {
    #expect(
      PostcodeEntryStepView.Copy.helper
        != "Enter your postcode so we can find planning applications near you."
    )
  }

  @Test func helper_exactWording() {
    #expect(
      PostcodeEntryStepView.Copy.helper
        == "Any UK postcode works. Your home, your work, a relative's street, or a "
        + "site you're keeping an eye on. We'll find planning applications nearby."
    )
  }

  // MARK: - Placeholder

  @Test func placeholder_showsValidExamplePostcode() {
    #expect(PostcodeEntryStepView.Copy.placeholder == "e.g. CB1 2AD")
  }

  // MARK: - View

  @Test func body_renders() {
    let sut = PostcodeEntryStepView(viewModel: makeViewModel())
    _ = sut.body
  }
}
