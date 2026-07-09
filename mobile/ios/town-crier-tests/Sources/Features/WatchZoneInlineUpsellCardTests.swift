import Testing

@testable import TownCrierDomain
@testable import TownCrierPresentation

@MainActor
@Suite("WatchZoneInlineUpsellCard")
struct WatchZoneInlineUpsellCardTests {

  // The inline card sells the whole upgrade beneath a free user's single zone.
  // Copy is exposed as constants so the value proposition lives in one place and
  // can be asserted directly, and reuses the tc-42gc wording so both surfaces
  // speak in one voice.

  // MARK: - Heading

  @Test func eyebrow_readsUpgrade() {
    // The brass small-caps eyebrow (GH#857) — the only filled-amber
    // container on this screen is the CTA button below; the card itself
    // stays bordered, not filled (amber-rationing rule).
    #expect(WatchZoneInlineUpsellCard.Copy.eyebrow == "Upgrade")
  }

  @Test func title_invitesAnUpgrade() {
    #expect(WatchZoneInlineUpsellCard.Copy.title == "Do more with a plan")
  }

  // MARK: - Benefit rows

  @Test func benefit_biggerZones() {
    #expect(WatchZoneInlineUpsellCard.Copy.biggerZones == "Bigger watch zones, up to 10 km")
  }

  @Test func benefit_moreThanOneZone() {
    #expect(WatchZoneInlineUpsellCard.Copy.moreThanOneZone == "More than one watch zone")
  }

  @Test func benefit_instantAlertsAreOneAlertTwoChannels() {
    // Push + email are two channels for the SAME alert, never two features.
    #expect(WatchZoneInlineUpsellCard.Copy.instantAlerts == "Instant alerts by push and email")
  }

  @Test func freeClarifier_explainsTheFreeTier() {
    #expect(WatchZoneInlineUpsellCard.Copy.freeClarifier == "Free gives you a weekly digest.")
  }

  // MARK: - CTA

  @Test func viewPlans_ctaWording() {
    #expect(WatchZoneInlineUpsellCard.Copy.viewPlans == "View Plans")
  }

  // MARK: - Accessibility

  @Test func accessibilityHint_opensPlans() {
    #expect(WatchZoneInlineUpsellCard.Copy.accessibilityHint == "Opens subscription plans")
  }

  @Test func accessibilityLabel_describesTheWholeUpgrade() {
    let label = WatchZoneInlineUpsellCard.Copy.accessibilityLabel
    #expect(label.contains("Do more with a plan"))
    #expect(label.contains("Bigger watch zones, up to 10 kilometres"))
    #expect(label.contains("More than one watch zone"))
    #expect(label.contains("Instant alerts by push and email"))
    #expect(label.contains("Free gives you a weekly digest."))
  }

  // MARK: - View

  @Test func body_renders() {
    let sut = WatchZoneInlineUpsellCard {}
    _ = sut.body
  }

  // MARK: - Callback

  @Test func onViewPlans_isCalled_whenViewPlansTapped() {
    var called = false
    let sut = WatchZoneInlineUpsellCard { called = true }

    sut.simulateViewPlansTap()

    #expect(called)
  }

  @Test func viewPlansTap_triggersViewModelViewPlans() async {
    let spy = SpyWatchZoneRepository()
    let viewModel = WatchZoneListViewModel(
      repository: spy,
      featureGate: FeatureGate(tier: .free)
    )
    var viewPlansCalled = false
    viewModel.onViewPlans = { viewPlansCalled = true }
    let sut = WatchZoneInlineUpsellCard { viewModel.viewPlans() }

    sut.simulateViewPlansTap()

    #expect(viewPlansCalled)
    #expect(!viewModel.isUpgradePromptPresented)
  }
}
